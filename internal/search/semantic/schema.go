package semantic

import (
	"context"
	"crypto/sha256"
	"database/sql" //nolint:depguard // SQLite is consumed through the standard database/sql boundary.
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"strings"
)

// expectedStoreSchemaFingerprint seals schema v1's tables, constraints,
// foreign keys, and explicit indexes. Comments and whitespace are removed
// before hashing so documentation-only edits do not invalidate paid vectors.
var expectedStoreSchemaFingerprint = [sha256.Size]byte{
	0x23, 0xf6, 0xa7, 0xb0, 0x53, 0x37, 0x35, 0x03,
	0x5e, 0xfa, 0xcd, 0x8e, 0xfa, 0xb5, 0xf7, 0xfc,
	0x02, 0xdc, 0x07, 0x12, 0x75, 0x2c, 0xa5, 0x5b,
	0xa7, 0x8e, 0x44, 0x95, 0x2f, 0x84, 0xd1, 0x95,
}

// schemaObjectsSQL is part of the recorded pre-schema sqlite_schema exception:
// sqlc cannot analyze SQLite's built-in catalog, while generated domain queries
// must not run until this check has proved the expected schema exists.
const schemaObjectsSQL = `
SELECT type, name, tbl_name, sql
FROM sqlite_schema
WHERE name NOT LIKE 'sqlite_%'
ORDER BY type, name
`

func requireExactStoreSchema(ctx context.Context, db *sql.DB) error {
	fingerprint, err := storeSchemaFingerprint(ctx, db)
	if err != nil {
		return fmt.Errorf("%w: fingerprint schema: %w", ErrStoreCorrupt, err)
	}
	if fingerprint != expectedStoreSchemaFingerprint {
		return fmt.Errorf("%w: schema fingerprint", ErrStoreCorrupt)
	}
	return nil
}

func storeSchemaFingerprint(ctx context.Context, db *sql.DB) (fingerprint [sha256.Size]byte, resultErr error) {
	rows, err := db.QueryContext(ctx, schemaObjectsSQL)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, rows.Close())
	}()

	digest := sha256.New()
	for rows.Next() {
		var objectType, name, tableName string
		var statement sql.NullString
		if err := rows.Scan(&objectType, &name, &tableName, &statement); err != nil {
			return [sha256.Size]byte{}, err
		}
		writeStoreSchemaPart(digest, objectType)
		writeStoreSchemaPart(digest, name)
		writeStoreSchemaPart(digest, tableName)
		writeStoreSchemaPart(digest, canonicalSchemaSQL(statement.String))
	}
	if err := rows.Err(); err != nil {
		return [sha256.Size]byte{}, err
	}
	copy(fingerprint[:], digest.Sum(nil))
	return fingerprint, nil
}

func writeStoreSchemaPart(digest hash.Hash, value string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value))) // #nosec G115 -- string length is non-negative
	_, _ = digest.Write(size[:])
	_, _ = digest.Write([]byte(value))
}

// canonicalSchemaSQL removes comments and collapses SQL whitespace while
// preserving quoted literals and identifiers. It is intentionally not a SQL
// parser: SQLite already parsed the stored statement before sqlite_schema could
// contain it; this step only keeps non-semantic formatting out of the seal.
func canonicalSchemaSQL(statement string) string {
	return collapseSchemaWhitespace(stripSchemaComments(statement))
}

func stripSchemaComments(statement string) string {
	var result strings.Builder
	result.Grow(len(statement))
	for position := 0; position < len(statement); {
		position = writeSchemaFragment(&result, statement, position)
	}
	return result.String()
}

func writeSchemaFragment(result *strings.Builder, statement string, position int) int {
	current := statement[position]
	if end, quoted := schemaQuoteEnd(current); quoted {
		next := schemaQuotedEnd(statement, position+1, end)
		for index := position; index < next; index++ {
			result.WriteByte(statement[index])
		}
		return next
	}
	if strings.HasPrefix(statement[position:], "--") {
		result.WriteByte(' ')
		return schemaLineCommentEnd(statement, position+2)
	}
	if strings.HasPrefix(statement[position:], "/*") {
		result.WriteByte(' ')
		return schemaBlockCommentEnd(statement, position+2)
	}
	result.WriteByte(current)
	return position + 1
}

func schemaQuotedEnd(statement string, position int, quoteEnd byte) int {
	for position < len(statement) {
		if statement[position] != quoteEnd {
			position++
			continue
		}
		position++
		if position >= len(statement) || statement[position] != quoteEnd {
			return position
		}
		position++
	}
	return position
}

func schemaLineCommentEnd(statement string, position int) int {
	for position < len(statement) && statement[position] != '\n' {
		position++
	}
	return position
}

func schemaBlockCommentEnd(statement string, position int) int {
	for position+1 < len(statement) {
		if statement[position] == '*' && statement[position+1] == '/' {
			return position + 2
		}
		position++
	}
	return len(statement)
}

func collapseSchemaWhitespace(statement string) string {
	var result strings.Builder
	result.Grow(len(statement))
	spacePending := false
	var quoteEnd byte
	for i := 0; i < len(statement); i++ {
		current := statement[i]
		if quoteEnd == 0 && isSchemaSpace(current) {
			spacePending = true
			continue
		}
		if spacePending && result.Len() != 0 {
			result.WriteByte(' ')
		}
		spacePending = false
		result.WriteByte(current)
		if quoteEnd == 0 {
			if end, quoted := schemaQuoteEnd(current); quoted {
				quoteEnd = end
			}
			continue
		}
		if current == quoteEnd {
			if i+1 < len(statement) && statement[i+1] == quoteEnd {
				i++
				result.WriteByte(statement[i])
				continue
			}
			quoteEnd = 0
		}
	}
	return strings.TrimSpace(result.String())
}

func schemaQuoteEnd(value byte) (byte, bool) {
	switch value {
	case '\'', '"', '`':
		return value, true
	case '[':
		return ']', true
	default:
		return 0, false
	}
}

func isSchemaSpace(value byte) bool {
	switch value {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	default:
		return false
	}
}
