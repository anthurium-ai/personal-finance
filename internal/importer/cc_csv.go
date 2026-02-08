package importer

import (
	"bufio"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// ImportCCCSV imports the credit card CSV format you pasted.
//
// It dedupes using row_hash (sha256 over canonical fields), so you can re-import safely.
func ImportCCCSV(db *sql.DB, r io.Reader, fileName string) (importID int64, total int, inserted int, skipped int, err error) {
	br := bufio.NewReader(r)
	cr := csv.NewReader(br)
	cr.FieldsPerRecord = -1

	header, err := cr.Read()
	if err != nil {
		return 0, 0, 0, 0, err
	}
	idx := indexMap(header)

	// Read the whole file content hash (for imports table).
	// We can't rewind easily here, so just hash canonical rows while reading.
	fileHash := sha256.New()

	tx, err := db.Begin()
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	res, err := tx.Exec(`INSERT INTO imports (source,file_name,sha256,rows_total,rows_inserted,rows_skipped) VALUES (?,?,?,?,?,?)`, "cc_csv", fileName, "", 0, 0, 0)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	importID, _ = res.LastInsertId()

	insStmt, err := tx.Prepare(`INSERT INTO transactions (
		import_id, txn_date, processed_on, amount_cents, account, txn_type, details, category_raw, merchant_raw,
		merchant_norm, category_norm, row_hash
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer insStmt.Close()

	for {
		row, err2 := cr.Read()
		if err2 == io.EOF {
			break
		}
		if err2 != nil {
			err = err2
			return
		}
		total++

		get := func(name string) string {
			i, ok := idx[strings.ToLower(name)]
			if !ok || i < 0 || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}

		dateStr := get("Date")
		amtStr := get("Amount")
		acct := get("Account Number")
		txnType := get("Transaction Type")
		details := get("Transaction Details")
		cat := get("Category")
		merchant := get("Merchant Name")
		processedOn := get("Processed On")

		txnDate, err2 := parseAUDate(dateStr)
		if err2 != nil {
			skipped++
			continue
		}
		amountCents, err2 := parseAmountCents(amtStr)
		if err2 != nil {
			skipped++
			continue
		}

		merchantNorm := strings.TrimSpace(merchant)
		catNorm := strings.TrimSpace(cat)

		rowHash := hashRow(txnDate.Format("2006-01-02"), processedOn, amountCents, acct, txnType, details, cat, merchant)
		fileHash.Write([]byte(rowHash))

		_, err2 = insStmt.Exec(importID, txnDate.Format("2006-01-02"), processedOn, amountCents, acct, txnType, details, cat, merchant, merchantNorm, catNorm, rowHash)
		if err2 != nil {
			// unique constraint => already imported
			if strings.Contains(err2.Error(), "UNIQUE") {
				skipped++
				continue
			}
			err = err2
			return
		}
		inserted++
	}

	sha := hex.EncodeToString(fileHash.Sum(nil))
	_, err = tx.Exec(`UPDATE imports SET sha256=?, rows_total=?, rows_inserted=?, rows_skipped=? WHERE id=?`, sha, total, inserted, skipped, importID)
	if err != nil {
		return
	}

	err = tx.Commit()
	return
}

func indexMap(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		m[strings.ToLower(h)] = i
	}
	return m
}

func parseAUDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	// Example: 07 Feb 26
	return time.Parse("02 Jan 06", s)
}

func parseAmountCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty amount")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	// convert dollars to cents
	c := int64(f * 100)
	return c, nil
}

func hashRow(txnDate string, processedOn string, amountCents int64, acct string, txnType string, details string, cat string, merchant string) string {
	canon := strings.Join([]string{
		"date=" + txnDate,
		"processed=" + processedOn,
		"amount_cents=" + strconv.FormatInt(amountCents, 10),
		"acct=" + acct,
		"type=" + txnType,
		"details=" + details,
		"cat=" + cat,
		"merchant=" + merchant,
	}, "\n")
	sum := sha256.Sum256([]byte(canon))
	return hex.EncodeToString(sum[:])
}
