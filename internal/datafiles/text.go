package datafiles

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func delimiter(o Options) (rune, error) {
	if o.Format == "csv" {
		return ',', nil
	}
	switch o.Delimiter {
	case "", "tab", "\t":
		return '\t', nil
	case ",":
		return ',', nil
	case ";":
		return ';', nil
	case "|":
		return '|', nil
	}
	return 0, errors.New("请选择制表符、逗号、分号或竖线分隔")
}
func decodeText(raw []byte, o Options) (Data, error) {
	if !utf8.Valid(raw) {
		return Data{}, errors.New("文本文件必须使用 UTF-8 编码")
	}
	raw = bytes.TrimPrefix(raw, []byte{239, 187, 191})
	switch o.Format {
	case "json":
		return readJSON(raw)
	case "xml":
		return readXML(raw)
	}
	sep, err := delimiter(o)
	if err != nil {
		return Data{}, err
	}
	return readDelimited(raw, byte(sep))
}
func readDelimited(raw []byte, sep byte) (Data, error) {
	d := Data{Rows: [][]*string{}}
	var record []string
	var field strings.Builder
	quoted, closed, started := false, false, false
	pushField := func() error {
		record = append(record, field.String())
		field.Reset()
		closed = false
		started = false
		if len(record) > MaxColumns {
			return errors.New("每行最多 1,024 列")
		}
		return nil
	}
	pushRow := func() error {
		if err := pushField(); err != nil {
			return err
		}
		if d.Columns == nil {
			d.Columns = record
		} else {
			if len(record) != len(d.Columns) {
				return errors.New("每行列数必须与字段名一致")
			}
			row := make([]*string, len(record))
			for i, v := range record {
				row[i] = textCell(v)
			}
			d.Rows = append(d.Rows, row)
			if len(d.Rows) > MaxRows || len(d.Rows)*len(d.Columns) > 200000 {
				return errors.New("文本数据行数或单元格数超限，已停止而非截断")
			}
		}
		record = nil
		return nil
	}
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if quoted {
			if c == '"' {
				if i+1 < len(raw) && raw[i+1] == '"' {
					field.WriteByte('"')
					i++
				} else {
					quoted = false
					closed = true
				}
			} else {
				field.WriteByte(c)
			}
			continue
		}
		if c == sep {
			if err := pushField(); err != nil {
				return Data{}, err
			}
			continue
		}
		if c == '\r' || c == '\n' {
			if c == '\r' && i+1 < len(raw) && raw[i+1] == '\n' {
				i++
			}
			if err := pushRow(); err != nil {
				return Data{}, err
			}
			continue
		}
		if closed {
			return Data{}, errors.New("引号结束后只能是分隔符或换行")
		}
		if c == '"' {
			if started {
				return Data{}, errors.New("字段中的引号必须转义")
			}
			quoted = true
			started = true
		} else {
			field.WriteByte(c)
			started = true
		}
	}
	if quoted {
		return Data{}, errors.New("文本引号未闭合")
	}
	if started || closed || field.Len() > 0 || len(record) > 0 {
		if err := pushRow(); err != nil {
			return Data{}, err
		}
	}
	return d, Validate(d)
}

func encodeText(d Data, o Options) ([]byte, error) {
	switch o.Format {
	case "json":
		if d.Rows == nil {
			d.Rows = [][]*string{}
		}
		return json.MarshalIndent(d, "", "  ")
	case "xml":
		x := xmlData{Columns: d.Columns}
		for _, row := range d.Rows {
			cells := make([]xmlCell, len(row))
			for i, c := range row {
				if c == nil {
					cells[i].Null = "true"
				} else {
					cells[i].Value = *c
				}
			}
			x.Rows = append(x.Rows, xmlRow{Cells: cells})
		}
		data, err := xml.MarshalIndent(x, "", "  ")
		return append([]byte(xml.Header), data...), err
	}
	sep, err := delimiter(o)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.WriteString("\ufeff")
	writeRow := func(values []string) {
		for i, v := range values {
			if i > 0 {
				out.WriteRune(sep)
			}
			out.WriteByte('"')
			out.WriteString(strings.ReplaceAll(v, `"`, `""`))
			out.WriteByte('"')
		}
		out.WriteString("\r\n")
	}
	headers := append([]string(nil), d.Columns...)
	if o.ProtectFormulas {
		for i, v := range headers {
			headers[i] = protectFormula(v)
		}
	}
	writeRow(headers)
	for _, row := range d.Rows {
		values := make([]string, len(row))
		for i, c := range row {
			value := cellText(c)
			if o.ProtectFormulas && c != nil {
				value = protectFormula(value)
			}
			values[i] = value
		}
		writeRow(values)
	}
	return out.Bytes(), nil
}

func protectFormula(value string) string {
	if strings.ContainsAny(firstRune(value), "\t\r\n") || strings.ContainsAny(firstRune(strings.TrimLeft(value, " \t\r\n")), "=+-@") {
		return "'" + value
	}
	return value
}
func firstRune(s string) string {
	if s == "" {
		return ""
	}
	r, _ := utf8.DecodeRuneInString(s)
	return string(r)
}
func jsonCell(value any) (*string, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case string:
		return text(v), nil
	case json.Number:
		return text(v.String()), nil
	case bool:
		return text(strconv.FormatBool(v)), nil
	default:
		raw, err := json.Marshal(v)
		return text(string(raw)), err
	}
}
func readJSON(raw []byte) (Data, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	nodes := 0
	root, err := jsonValue(decoder, 0, &nodes)
	if err != nil {
		return Data{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Data{}, errors.New("JSON 必须只有一个完整文档")
	}
	if object, ok := root.(map[string]any); ok {
		headers, hok := object["columns"].([]any)
		records, rok := object["rows"].([]any)
		if !hok || !rok {
			return Data{}, errors.New("JSON 数据对象需要 columns 和 rows 数组")
		}
		d := Data{Rows: [][]*string{}}
		for _, h := range headers {
			v, ok := h.(string)
			if !ok {
				return Data{}, errors.New("columns 必须是字符串数组")
			}
			d.Columns = append(d.Columns, v)
		}
		if len(records) > MaxRows {
			return Data{}, errors.New("JSON 超过 10,000 行")
		}
		for _, record := range records {
			values, ok := record.([]any)
			if !ok {
				return Data{}, errors.New("rows 必须是二维数组")
			}
			row := make([]*string, len(values))
			for i, value := range values {
				cell, err := jsonCell(value)
				if err != nil {
					return Data{}, err
				}
				row[i] = cell
			}
			d.Rows = append(d.Rows, row)
		}
		return d, Validate(d)
	}
	records, ok := root.([]any)
	if !ok || len(records) == 0 {
		return Data{}, errors.New("JSON 需要非空对象数组，或 columns/rows 数据对象")
	}
	if len(records) > MaxRows {
		return Data{}, errors.New("JSON 超过 10,000 行")
	}
	keys := map[string]bool{}
	for _, r := range records {
		object, ok := r.(map[string]any)
		if !ok {
			return Data{}, errors.New("JSON 数组中的每项必须是对象")
		}
		for k := range object {
			keys[k] = true
		}
		if len(keys) > MaxColumns {
			return Data{}, errors.New("JSON 列数超限")
		}
	}
	d := Data{Rows: [][]*string{}}
	for k := range keys {
		d.Columns = append(d.Columns, k)
	}
	sort.Strings(d.Columns)
	for _, record := range records {
		object := record.(map[string]any)
		row := make([]*string, len(d.Columns))
		for i, col := range d.Columns {
			cell, err := jsonCell(object[col])
			if err != nil {
				return Data{}, err
			}
			row[i] = cell
		}
		d.Rows = append(d.Rows, row)
	}
	return d, Validate(d)
}

func jsonValue(decoder *json.Decoder, depth int, nodes *int) (any, error) {
	*nodes++
	if depth > 32 || *nodes > 400000 {
		return nil, errors.New("JSON 嵌套或节点数量超限")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delim {
	case '{':
		object := map[string]any{}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			name, ok := key.(string)
			if !ok {
				return nil, errors.New("JSON 对象字段名无效")
			}
			if _, exists := object[name]; exists {
				return nil, errors.New("JSON 对象包含重复字段：" + name)
			}
			if len(object) >= MaxColumns {
				return nil, errors.New("JSON 对象字段数超限")
			}
			value, err := jsonValue(decoder, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			object[name] = value
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			if len(array) >= 200000 {
				return nil, errors.New("JSON 数组超限")
			}
			value, err := jsonValue(decoder, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return array, nil
	}
	return nil, errors.New("JSON 结构无效")
}

type xmlCell struct {
	Null  string `xml:"null,attr,omitempty"`
	Value string `xml:",chardata"`
}
type xmlRow struct {
	Cells []xmlCell `xml:"cell"`
}
type xmlData struct {
	XMLName xml.Name `xml:"data"`
	Columns []string `xml:"columns>column"`
	Rows    []xmlRow `xml:"rows>row"`
}

func readXML(raw []byte) (Data, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	depth := 0
	roots := 0
	var columns []string
	var records []map[string]*string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Data{}, err
		}
		switch node := token.(type) {
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(node)) != "" {
				return Data{}, errors.New("XML 根元素之外不能有文本")
			}
		case xml.Directive:
			return Data{}, errors.New("XML 不支持 DTD 或外部实体")
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots > 1 {
					return Data{}, errors.New("XML 必须只有一个根元素")
				}
			}
			depth++
			if depth > 32 {
				return Data{}, errors.New("XML 嵌套过深")
			}
			if node.Name.Local == "column" && depth == 3 {
				value, err := xmlScalar(decoder, node)
				if err != nil {
					return Data{}, err
				}
				columns = append(columns, value)
				depth--
				if len(columns) > MaxColumns {
					return Data{}, errors.New("XML 列数超限")
				}
				continue
			}
			if node.Name.Local != "row" && node.Name.Local != "record" {
				continue
			}
			row := map[string]*string{}
			index := 0
			for {
				t, e := decoder.Token()
				if e != nil {
					return Data{}, e
				}
				if end, ok := t.(xml.EndElement); ok && end.Name == node.Name {
					break
				}
				start, ok := t.(xml.StartElement)
				if !ok {
					continue
				}
				name := start.Name.Local
				null := false
				for _, attr := range start.Attr {
					if attr.Name.Local == "name" {
						name = attr.Value
					}
					if (attr.Name.Local == "null" || attr.Name.Local == "nil") && (attr.Value == "true" || attr.Value == "1") {
						null = true
					}
				}
				if name == "cell" {
					if index >= len(columns) {
						return Data{}, errors.New("XML cell 需要对应 columns 字段名")
					}
					name = columns[index]
				}
				if _, exists := row[name]; exists {
					return Data{}, errors.New("XML 行含重复字段")
				}
				var value string
				for {
					content, e := decoder.Token()
					if e != nil {
						return Data{}, e
					}
					switch v := content.(type) {
					case xml.StartElement:
						return Data{}, errors.New("XML 字段必须是文本，不能嵌套元素")
					case xml.CharData:
						value += string(v)
					case xml.EndElement:
						if v.Name == start.Name {
							goto fieldDone
						}
					}
				}
			fieldDone:
				if null {
					row[name] = nil
				} else {
					row[name] = text(value)
				}
				index++
				if index > MaxColumns {
					return Data{}, errors.New("XML 列数超限")
				}
			}
			records = append(records, row)
			depth--
			if len(records) > MaxRows {
				return Data{}, errors.New("XML 超过 10,000 行")
			}
		case xml.EndElement:
			depth--
		}
	}
	if len(columns) == 0 {
		keys := map[string]bool{}
		for _, r := range records {
			for k := range r {
				keys[k] = true
			}
		}
		for k := range keys {
			columns = append(columns, k)
		}
		sort.Strings(columns)
	}
	d := Data{Columns: columns, Rows: [][]*string{}}
	for _, r := range records {
		row := make([]*string, len(columns))
		for i, c := range columns {
			row[i] = r[c]
		}
		for key := range r {
			found := false
			for _, name := range columns {
				if key == name {
					found = true
					break
				}
			}
			if !found {
				return Data{}, errors.New("XML 字段与标题不一致")
			}
		}
		d.Rows = append(d.Rows, row)
	}
	return d, Validate(d)
}

func xmlScalar(decoder *xml.Decoder, start xml.StartElement) (string, error) {
	var value strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch node := token.(type) {
		case xml.CharData:
			value.Write(node)
		case xml.StartElement, xml.Directive:
			return "", errors.New("XML 字段名必须是文本，不能嵌套元素或实体声明")
		case xml.EndElement:
			if node.Name == start.Name {
				return value.String(), nil
			}
		}
	}
}
