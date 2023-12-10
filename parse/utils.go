package parse

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type PreProcessor func([]byte) []byte

func removeComments(data []byte) []byte {
	var result []byte
	inComment := false

	for i := 0; i < len(data); i++ {
		switch {
		case inComment && data[i] == '\n':
			inComment = false
			//Don't append the newline if it's the start of the data
			if i != 0 {
				result = append(result, '\n')
			}
		case !inComment && data[i] == '#':
			inComment = true
		case !inComment:
			result = append(result, data[i])
		}
	}

	//result = bytes.TrimSuffix(result, []byte("\n"))
	return result
}

func noPreProcessing(data []byte) []byte {
	return data
}

// function for reading the bytes into memory
func outputInMemory(inputPath string, fn func(string) (*os.File, error), preprocessor PreProcessor) ([]byte, error) {

	inputFile, err := fn(inputPath)

	if err != nil {
		return nil, err
	}

	defer inputFile.Close()

	reader := bufio.NewReader(inputFile)

	var output []byte

	for {
		chunk, err := reader.ReadBytes('\n')
		if err != nil {
			// Handle errors other than EOF
			if err != io.EOF {
				return nil, err
			}
			if len(chunk) == 0 {
				break
			}
		}

		processedChunk := preprocessor(chunk)
		output = append(output, processedChunk...)

		if err == io.EOF {
			break
		}
	}

	return output, nil

}

func extractFileName(path string) (string, error) {
	b := filepath.Base(path)
	if b == "." {
		return "", fmt.Errorf("path is empty: %s", path)
	}
	if b == "/" {
		return "", fmt.Errorf("no file in path: %s", path)
	}
	return fmt.Sprintf("copy_%s", b), nil
}

func processInputPath(inputPath string) (string, error) {

	ip, err := filepath.Abs(inputPath)

	if err != nil {
		return "", err
	}

	return ip, nil

}

func processInputFile(inputPath string) (*os.File, error) {
	ip, err := processInputPath(inputPath)
	if err != nil {
		return nil, err
	}
	inputPath = ip
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	return inputFile, nil
}

func processFilePaths(inputPath, outputDir string) (*os.File, *os.File, error) {

	ip, err := processInputPath(inputPath)

	if err != nil {
		return nil, nil, err
	}

	inputPath = ip

	outFileName, err := extractFileName(inputPath)

	if err != nil {
		return nil, nil, err
	}

	outPath := filepath.Join(outputDir, outFileName)

	if err != nil {
		return nil, nil, err
	}

	inputFile, err := os.Open(inputPath)

	if err != nil {
		return nil, nil, err
	}

	outputFile, err := os.Create(outPath)

	// remember to close the input if error in the output file!!!!
	if err != nil {
		inputFile.Close()
		return nil, nil, err
	}

	return inputFile, outputFile, nil

}

type fieldStruct interface {
	Name() string
	ValueType() string
}

type mapField struct {
	name      string
	fields    map[string]fieldStruct
	valueType string
}

func (mf *mapField) Name() string {
	return mf.name
}

func (mf *mapField) ValueType() string {
	return mf.valueType
}

func newMapField(name, valueType string) *mapField {
	return &mapField{name: name, fields: make(map[string]fieldStruct), valueType: valueType}
}

func (mf *mapField) fieldKeys() []string {
	keys := make([]string, 0)
	for key := range mf.fields {
		keys = append(keys, key)
	}
	return keys
}

func (mf *mapField) keyExists(key string) bool {
	_, exists := mf.fields[key]
	return exists
}

func (mf *mapField) addFields(field *mapField) {
	for key, value := range field.fields {
		if mf.keyExists(key) {
			switch value.(type) {
			case *mapField:
				mf.fields[key].(*mapField).addFields(value.(*mapField))
			case *listField:
				mf.fields[key].(*listField).checkValueAdd(value.(*listField))
			default:
				continue
			}
		} else {
			mf.fields[key] = value
		}
	}
}

type listField struct {
	name      string
	values    []fieldStruct
	valueType string
}

func newListField(name, valueType string) *listField {
	return &listField{name: name, values: make([]fieldStruct, 0), valueType: valueType}
}

func (lf *listField) Name() string {
	return lf.name
}

func (lf *listField) ValueType() string {
	return lf.valueType
}

func (lf *listField) checkValueAdd(value fieldStruct) {
	missing := true
	for _, v := range lf.values {
		switch value.(type) {
		case *mapField:
			if v.Name() == value.Name() {
				if v2, ok := value.(*mapField); ok {
					v.(*mapField).addFields(v2)
					missing = false
				}
			}
		case *listField:
			if v.Name() == value.Name() {
				if v2, ok := value.(*listField); ok {
					lf.hasValueOrAdd(v2)
					missing = false
				}
			}
		case *scalarField:
			if v.Name() == value.Name() {
				if v2, ok := value.(*scalarField); ok {
					if v.(*scalarField).name != v2.name {
						lf.values = append(lf.values, value)
						missing = false
					}
				}
			}
		}
	}
	if missing {
		lf.values = append(lf.values, value)
	}
}

func (lf *listField) hasValueOrAdd(second *listField) {
	for _, v := range second.values {
		missing := true
		for _, v2 := range lf.values {
			if v.Name() == v2.Name() {
				missing = false
			}
		}
		if missing {
			lf.values = append(lf.values, v)
		}
	}
}

type scalarField struct {
	name      string
	value     interface{}
	valueType string
}

func newScalarField(name, valueType string) *scalarField {
	return &scalarField{name: name, valueType: valueType}
}

func (sf *scalarField) ValueType() string {
	return sf.valueType
}

func (df *scalarField) Name() string {
	return df.name
}

func indent(depth int) string {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	return indent
}

func printNodes(n fieldStruct, depth int) {
	switch v := n.(type) {
	case *mapField:

		for key, value := range v.fields {
			fmt.Printf("%s %s - %s\n", indent(depth), key, value.ValueType())
			printNodes(value, depth+1)
		}
	case *listField:
		for _, value := range v.values {
			fmt.Printf("%s%s - %s:\n", indent(depth), value.Name(), value.ValueType())
			printNodes(value, depth+1)
		}
	case *scalarField:
		fmt.Printf("%s %s: %v - %s\n", indent(depth), v.Name(), v.value, v.valueType)
	}
}
