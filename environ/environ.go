// Package environ 读取设置环境变量相关方法
package environ

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const doubleQuoteSpecialChars = "\\\n\r\"!$`"

var (
	expandVarRegex = regexp.MustCompile(`(\\)?(\$)(\()?\{?([A-Z0-9_]+)?\}?`)
	exportRegex    = regexp.MustCompile(`^\s*(?:export\s+)?(.*?)\s*$`)

	singleQuotesRegex  = regexp.MustCompile(`\A'(.*)'\z`)
	doubleQuotesRegex  = regexp.MustCompile(`\A"(.*)"\z`)
	escapeRegex        = regexp.MustCompile(`\\.`)
	unescapeCharsRegex = regexp.MustCompile(`\\([^$])`)
)

// ViperParse viper解析方法
// @param  filepath  string  配置文件路径
func ViperParse(conf any, filepath string) error {
	cv := viper.New()
	extension := "yaml"                                // 设置默认配置文件类型
	if s := strings.Split(filepath, "."); len(s) > 1 { // 截取文件扩展名
		extension = s[len(s)-1]
	}
	cv.SetConfigType(extension)
	cv.SetConfigFile(filepath)

	// 读取并加载配置文件
	if err := cv.ReadInConfig(); err != nil {
		return err
	}

	if err := cv.Unmarshal(conf); err != nil {
		return err
	}

	return nil
}

// Environs 返回环境变量键值对："key=value"
func Environs() []string {
	return os.Environ()
}

// DoesFileExists 判读一个文件是否存在
// @param   path  string  文件路径
// @return  bool 存在则为true
func DoesFileExists(filepath string) bool {
	if _, err := os.Stat(filepath); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// LoadDotenv 从文件加载环境变量, 不覆盖系统环境变量
// @param   filepath  string  环境变量文件路径
// @return  error 如果文件不存在则返回错误
func LoadDotenv(filepath string) error {
	if !DoesFileExists(filepath) {
		return errors.New("File does not exist: " + filepath)
	}
	if err := loadFile(filepath, false); err != nil {
		return err
	} else {
		return nil
	}
}

// AdvancedLoadDotenv 加载环境变量文件, 是 LoadDotenv 方法的增强版
// @param   filepath  string     环境变量文件路径
// @param   stream    io.Reader  IO读取接口
// @param   override  bool       是否覆盖系统环境变量
// @return  error 如果文件不存在则返回错误
func AdvancedLoadDotenv(filepath string, stream io.Reader, override bool) error {
	if filepath == "" && stream == nil {
		return errors.New("stream is nil and File does not exist: " + filepath)
	}

	if filepath != "" { // 从文件加载
		if err := loadFile(filepath, override); err != nil {
			return err
		} else {
			return nil
		}

	} else { // 从IO读取
		if envMap, err := Parse(stream); err != nil {
			return err
		} else {
			currentEnv := map[string]bool{}
			rawEnv := os.Environ()
			for _, rawEnvLine := range rawEnv {
				key := strings.Split(rawEnvLine, "=")[0]
				currentEnv[key] = true
			}

			for key, value := range envMap {
				if !currentEnv[key] || override {
					_ = os.Setenv(key, value)
				}
			}

			return nil
		}
	}
}

// GetInt 读取int类型环境变量，若未提供默认值，则在未读取到环境变量时抛出错误，
// 若提供了多个默认值，则对第一个默认值做int类型转换后并返回。
func GetInt(key string, args ...any) int {
	if k, err := strconv.Atoi(os.Getenv(key)); err != nil {
		if len(args) == 0 {
			panic("no environment variable was found: " + key)
		}
		first := args[0] // 取第一个默认值
		switch first.(type) {

		case int8:
			return int(first.(int8))
		case int16:
			return int(first.(int16))
		case int32:
			return int(first.(int32))
		case int64:
			return int(first.(int64))
		case int:
			return first.(int)

		case byte:
			return int(first.(byte))
		case uint16:
			return int(first.(uint16))
		case uint32:
			return int(first.(uint32))
		case uint64:
			return int(first.(uint64))
		case uint:
			return int(first.(uint))

		case []byte:
			n, _ := strconv.ParseInt(string(first.([]byte)), 10, 0)
			return int(n)

		case string:
			if v, err := strconv.Atoi(first.(string)); err != nil {
				panic("cannot convert string to int: " + first.(string))
			} else {
				return v
			}

		default:
			panic("unknown type of args")
		}
	} else {
		return k
	}
}

// GetString 读取string类型环境变量
func GetString(key string, args ...any) string {
	if v := os.Getenv(key); v == "" {
		if len(args) == 0 {
			panic("no environment variable was read: " + key)
		}
		first := args[0]
		switch first.(type) {
		case string:
			return first.(string)

		case int8:
			return strconv.Itoa(int(first.(int8)))
		case int16:
			return strconv.Itoa(int(first.(int16)))
		case int32:
			return strconv.Itoa(int(first.(int32)))
		case int64:
			return strconv.Itoa(int(first.(int64)))
		case int:
			return strconv.Itoa(first.(int))

		case byte:
			return strconv.Itoa(int(first.(byte)))
		case uint16:
			return strconv.Itoa(int(first.(uint16)))
		case uint32:
			return strconv.Itoa(int(first.(uint32)))
		case uint64:
			return strconv.Itoa(int(first.(uint64)))
		case uint:
			return strconv.Itoa(int(first.(uint)))

		case []byte:
			return string(first.([]byte))
		case []string:
			return strings.Join(first.([]string), "")
		default:
			panic("unknown type of args")
		}

	} else {
		return v
	}
}

// GetBool returns true if key is not false or empty
func GetBool(key string, args ...any) bool {
	stringValue := GetString(key, args...)
	switch stringValue {
	case "false", "False", "0":
		return false
	case "true", "True", "1", "ok":
		return true
	default:
		return false
	}
}

// Read 读取环境变量文件并返回变量键值对，但并不设置到环境变量
func Read(filenames ...string) (envMap map[string]string, err error) {
	filenames = filenamesOrDefault(filenames)
	envMap = make(map[string]string)

	for _, filename := range filenames {
		individualEnvMap, individualErr := readFile(filename)

		if individualErr != nil {
			err = individualErr
			return // return early on a spazout
		}

		for key, value := range individualEnvMap {
			envMap[key] = value
		}
	}

	return
}

// Parse 从io.Reader中读取并解析环境变量, 返回环境变量键值对
func Parse(r io.Reader) (envMap map[string]string, err error) {
	envMap = make(map[string]string)

	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err = scanner.Err(); err != nil {
		return
	}

	for _, fullLine := range lines {
		if !isIgnoredLine(fullLine) {
			var key, value string
			key, value, err = parseLine(fullLine, envMap)

			if err != nil {
				return
			}
			envMap[key] = value
		}
	}
	return
}

// Unmarshal 从环境变量字符串中反序列化环境变量, 并返回键值对
func Unmarshal(str string) (envMap map[string]string, err error) {
	return Parse(strings.NewReader(str))
}

// Write 序列化环境变量并写入到文件
func Write(envMap map[string]string, filename string) error {
	content, err := Marshal(envMap)
	if err != nil {
		return err
	}
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(content + "\n")
	if err != nil {
		return err
	}
	_ = file.Sync()
	return err
}

// Marshal 将环境变量字典格式化为dotenv格式字符串,
// 其中每一行都形如: KEY="VALUE", 其中VALUE进行了反斜杠转义backslash-escaped
func Marshal(envMap map[string]string) (string, error) {
	lines := make([]string, 0, len(envMap))
	for k, v := range envMap {
		if d, err := strconv.Atoi(v); err == nil {
			lines = append(lines, fmt.Sprintf(`%s=%d`, k, d))
		} else {
			lines = append(lines, fmt.Sprintf(`%s="%s"`, k, doubleQuoteEscape(v)))
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), nil
}

func filenamesOrDefault(filenames []string) []string {
	if len(filenames) == 0 {
		return []string{".env"}
	}
	return filenames
}

func loadFile(filename string, overload bool) error {
	envMap, err := readFile(filename)
	if err != nil {
		return err
	}

	currentEnv := map[string]bool{}
	rawEnv := os.Environ()
	for _, rawEnvLine := range rawEnv {
		key := strings.Split(rawEnvLine, "=")[0]
		currentEnv[key] = true
	}

	for key, value := range envMap {
		if !currentEnv[key] || overload {
			_ = os.Setenv(key, value)
		}
	}

	return nil
}

func readFile(filename string) (envMap map[string]string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()

	return Parse(file)
}

func parseLine(line string, envMap map[string]string) (key string, value string, err error) {
	if len(line) == 0 {
		err = errors.New("zero length string")
		return
	}

	// ditch the comments (but keep quoted hashes)
	if strings.Contains(line, "#") {
		segmentsBetweenHashes := strings.Split(line, "#")
		quotesAreOpen := false
		var segmentsToKeep []string
		for _, segment := range segmentsBetweenHashes {
			if strings.Count(segment, "\"") == 1 || strings.Count(segment, "'") == 1 {
				if quotesAreOpen {
					quotesAreOpen = false
					segmentsToKeep = append(segmentsToKeep, segment)
				} else {
					quotesAreOpen = true
				}
			}

			if len(segmentsToKeep) == 0 || quotesAreOpen {
				segmentsToKeep = append(segmentsToKeep, segment)
			}
		}

		line = strings.Join(segmentsToKeep, "#")
	}

	firstEquals := strings.Index(line, "=")
	firstColon := strings.Index(line, ":")
	splitString := strings.SplitN(line, "=", 2)
	if firstColon != -1 && (firstColon < firstEquals || firstEquals == -1) {
		//this is a yaml-style line
		splitString = strings.SplitN(line, ":", 2)
	}

	if len(splitString) != 2 {
		err = errors.New("Can't separate key from value")
		return
	}

	// Parse the key
	key = splitString[0]
	if strings.HasPrefix(key, "export") {
		key = strings.TrimPrefix(key, "export")
	}
	key = strings.TrimSpace(key)

	key = exportRegex.ReplaceAllString(splitString[0], "$1")

	// Parse the value
	value = parseValue(splitString[1], envMap)
	return
}

func parseValue(value string, envMap map[string]string) string {

	// trim
	value = strings.Trim(value, " ")

	// check if we've got quoted values or possible escapes
	if len(value) > 1 {
		singleQuotes := singleQuotesRegex.FindStringSubmatch(value)

		doubleQuotes := doubleQuotesRegex.FindStringSubmatch(value)

		if singleQuotes != nil || doubleQuotes != nil {
			// pull the quotes off the edges
			value = value[1 : len(value)-1]
		}

		if doubleQuotes != nil {
			// expand newlines
			value = escapeRegex.ReplaceAllStringFunc(value, func(match string) string {
				c := strings.TrimPrefix(match, `\`)
				switch c {
				case "n":
					return "\n"
				case "r":
					return "\r"
				default:
					return match
				}
			})
			// unescape characters
			value = unescapeCharsRegex.ReplaceAllString(value, "$1")
		}

		if singleQuotes == nil {
			value = expandVariables(value, envMap)
		}
	}

	return value
}

func expandVariables(v string, m map[string]string) string {
	return expandVarRegex.ReplaceAllStringFunc(v, func(s string) string {
		submatch := expandVarRegex.FindStringSubmatch(s)

		if submatch == nil {
			return s
		}
		if submatch[1] == "\\" || submatch[2] == "(" {
			return submatch[0][1:]
		} else if submatch[4] != "" {
			return m[submatch[4]]
		}
		return s
	})
}

func isIgnoredLine(line string) bool {
	trimmedLine := strings.TrimSpace(line)
	return len(trimmedLine) == 0 || strings.HasPrefix(trimmedLine, "#")
}

func doubleQuoteEscape(line string) string {
	for _, c := range doubleQuoteSpecialChars {
		toReplace := "\\" + string(c)
		if c == '\n' {
			toReplace = `\n`
		}
		if c == '\r' {
			toReplace = `\r`
		}
		line = strings.Replace(line, string(c), toReplace, -1)
	}
	return line
}
