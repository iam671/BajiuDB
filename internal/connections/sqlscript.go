package connections

import (
	"errors"
	"strings"
	"unicode"
)

// SplitSQL honours quoted strings, identifiers, comments and mysql DELIMITER lines.
func SplitSQL(source string) ([]string, error) {
	return splitSQL(source, false, false)
}

// SplitSQLForMode keeps string boundaries consistent with the execution session.
func SplitSQLForMode(source, mode string) ([]string, error) {
	noBackslashEscapes, ansiQuotes := false, false
	for _, flag := range strings.Split(mode, ",") {
		if strings.EqualFold(strings.TrimSpace(flag), "ANSI_QUOTES") {
			ansiQuotes = true
		}
		if strings.EqualFold(strings.TrimSpace(flag), "NO_BACKSLASH_ESCAPES") {
			noBackslashEscapes = true
		}
	}
	return splitSQL(source, noBackslashEscapes, ansiQuotes)
}

func splitSQL(source string, noBackslashEscapes, ansiQuotes bool) ([]string, error) {
	if len(source) > 8<<20 {
		return nil, errors.New("SQL 文件超过 8 MB")
	}
	source = strings.TrimPrefix(source, "\ufeff")
	delimiter := ";"
	start := 0
	quote := byte(0)
	lineComment, blockComment := false, false
	lineStart := true
	statements := []string{}
	add := func(end int) error {
		v := strings.TrimSpace(source[start:end])
		if hasSQLCode(v) {
			statements = append(statements, v)
		}
		if len(statements) > 10000 {
			return errors.New("SQL 文件超过 10,000 条语句")
		}
		return nil
	}
	for i := 0; i < len(source); i++ {
		c := source[i]
		if lineComment {
			if c == '\n' {
				lineComment = false
				lineStart = true
			}
			continue
		}
		if blockComment {
			if c == '*' && i+1 < len(source) && source[i+1] == '/' {
				blockComment = false
				i++
			}
			continue
		}
		if quote != 0 {
			if c == '\\' && quote != '`' && !(ansiQuotes && quote == '"') && !noBackslashEscapes {
				i++
				continue
			}
			if c == quote {
				if i+1 < len(source) && source[i+1] == quote {
					i++
				} else {
					quote = 0
				}
			}
			continue
		}
		if lineStart {
			end := strings.IndexByte(source[i:], '\n')
			if end < 0 {
				end = len(source) - i
			}
			line := strings.TrimSpace(source[i : i+end])
			fields := strings.Fields(line)
			if len(fields) > 0 && strings.EqualFold(fields[0], "DELIMITER") {
				if hasSQLCode(source[start:i]) || len(fields) != 2 || len(fields[1]) > 16 {
					return nil, errors.New("DELIMITER 必须独占一行，且位于完整语句之间")
				}
				delimiter = fields[1]
				i += end
				start = i + 1
				lineStart = true
				continue
			}
			if !unicode.IsSpace(rune(c)) {
				lineStart = false
			}
		}
		if c == '\'' || c == '"' || c == '`' {
			quote = c
			continue
		}
		if c == '#' || (c == '-' && i+2 < len(source) && source[i+1] == '-' && unicode.IsSpace(rune(source[i+2]))) {
			lineComment = true
			continue
		}
		if c == '/' && i+1 < len(source) && source[i+1] == '*' {
			blockComment = true
			i++
			continue
		}
		if strings.HasPrefix(source[i:], delimiter) {
			if err := add(i); err != nil {
				return nil, err
			}
			i += len(delimiter) - 1
			start = i + 1
			continue
		}
		if c == '\n' {
			lineStart = true
		}
	}
	if quote != 0 || blockComment {
		return nil, errors.New("SQL 字符串、标识符或注释未闭合")
	}
	if start < len(source) {
		if err := add(len(source)); err != nil {
			return nil, err
		}
	}
	return statements, nil
}
func hasSQLCode(source string) bool {
	for {
		source = strings.TrimSpace(source)
		if source == "" {
			return false
		}
		if strings.HasPrefix(source, "#") || strings.HasPrefix(source, "-- ") || strings.HasPrefix(source, "--\t") {
			i := strings.IndexByte(source, '\n')
			if i < 0 {
				return false
			}
			source = source[i+1:]
			continue
		}
		if strings.HasPrefix(source, "/*") && !strings.HasPrefix(source, "/*!") {
			i := strings.Index(source, "*/")
			if i < 0 {
				return false
			}
			source = source[i+2:]
			continue
		}
		return true
	}
}
