package parse

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"

	"gopkg.in/yaml.v3"
)

func YAML(inputPath string) error {
	data, err := outputInMemory(inputPath, processInputFile, noPreProcessing)
	if err != nil {
		slog.Error("Error processing input file: %v", "error", err.Error(), "context", context.Background())
		return err
	}

	var rootNode yaml.Node

	err = yaml.Unmarshal(data, &rootNode)
	if err != nil {
		slog.Error("Error unmarshalling yaml: %v", "error", err.Error(), "context", context.Background())
		return err
	}

	children := make(map[string]fieldStruct)
	documentRoot := &mapField{name: "root", fields: children}

	for i := 0; i < len(rootNode.Content); i++ {
		err := handleDocumentNode(rootNode.Content[i], documentRoot)
		if err != nil {
			slog.Error("Error processing child node: %v", "error", err.Error())
		}
	}

	printNodes(documentRoot, 0)

	return nil

}

func handleDocumentNode(node *yaml.Node, parent *mapField) error {
	if node == nil {
		return fmt.Errorf("node is nil")
	}

	if len(node.Content) == 0 {
		return fmt.Errorf("node has no content")
	}

	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("node is not a mapping node")
	}

	for i := 0; i < len(node.Content); i++ {
		newIndex, child, err := handleContent(node, i, len(node.Content))
		i = newIndex

		if err != nil {
			slog.Error("Error processing child node: %v", "error", err.Error())
			return err
		}

		if child == nil {
			return fmt.Errorf("child is nil")
		}

		var node fieldStruct

		if parent.keyExists(child.Name()) {
			node = parent.fields[child.Name()]
		} else {
			parent.fields[child.Name()] = child
			continue
		}

		switch node.(type) {
		case *mapField:
			if cild, ok := child.(*mapField); ok {
				node.(*mapField).addFields(cild)
			}
		case *listField:
			if cild, ok := child.(*listField); ok {
				node.(*listField).checkValueAdd(cild)
			}
		case *scalarField:
			if cild, ok := child.(*scalarField); ok {
				if node.(*scalarField).value != cild.value {
					return fmt.Errorf("scalar field has different values")
				}
			}
		}
	}

	return nil
}

func handleContent(node *yaml.Node, i, length int) (int, fieldStruct, error) {
	switch node.Kind {
	case yaml.MappingNode:
		if i == length-1 {
			return i, handleMappingNode(node.Content[i], node.Content[i].Value), nil
		}
		child := handleMappingNode(node.Content[i+1], node.Content[i].Value)
		i++
		return i, child, nil
	case yaml.SequenceNode:
		if i == length-1 {
			return i, handleSequenceNode(node.Content[i], node.Content[i].Value), nil
		}
		child := handleSequenceNode(node.Content[i+1], node.Content[i].Value)
		i++
		return i, child, nil
	case yaml.ScalarNode:
		child := newScalarField(node.Tag, node.LongTag())
		return i, child, nil
	}
	// TODO have to follow up this and see how this affects different use cases and if it is correct
	return i, nil, fmt.Errorf("unknown node type")
}

func handleSequenceNode(node *yaml.Node, name string) *listField {
	lf := newListField(name, node.LongTag())
	for i := 0; i < len(node.Content); i++ {
		newIndex, child, err := handleContent(node, i, len(node.Content))
		i = newIndex
		if err != nil {
			slog.Error("Error processing child node: %v", "error", err.Error())
			continue
		}
		lf.checkValueAdd(child)
	}
	return lf
}

func handleMappingNode(node *yaml.Node, name string) *mapField {
	mf := &mapField{name: name, fields: make(map[string]fieldStruct), valueType: node.LongTag()}
	for i := 0; i < len(node.Content); i++ {
		newIndex, child, err := handleContent(node, i, len(node.Content))
		i = newIndex
		if err != nil {
			slog.Error("Error processing child node: %v", "error", err.Error())
			continue
		}
		if k, exists := mf.fields[name]; exists {
			switch k.(type) {
			case *mapField:
				if cild, ok := child.(*mapField); ok {
					k.(*mapField).addFields(cild)
				}
			case *listField:
				if cild, ok := child.(*listField); ok {
					k.(*listField).hasValueOrAdd(cild)
				}
			default:
				continue
			}
		} else {
			switch child.(type) {
			case *mapField:
				mf.fields[name] = child
			case *listField:
				mf.fields[name] = child
			case *scalarField:
				mf.fields[name] = child
			}
		}

	}
	return mf
}

// TODO potentially work on processing the yaml as a stream
func processYamlBuffer(reader *bufio.Reader, writer *bufio.Writer) error {
	for {
		// Read next chunk
		chunk, err := reader.ReadBytes('\n')

		if err != nil {
			if err != io.EOF {
				return err
			}
			if reader.Buffered() == 0 {

				if len(chunk) == 0 {
					break
				}
				if chunk[len(chunk)-1] == '\n' {
					chunk = chunk[:len(chunk)-1]
				}

			}

		}
		var processedChunk []byte

		for i := 0; i < len(chunk); i++ {
			switch {
			// Add cases here
			case chunk[i] == ':':
				fmt.Println("Before: ", string(chunk[:i]))
				fmt.Println("After: ", string(chunk[i+1:]))
			case chunk[i] == '-':

				if _, writeErr := writer.Write(processedChunk); writeErr != nil {
					return writeErr
				}

				if err == io.EOF {
					goto end
				}

			}

		}

	end:
		return nil

	}
	return nil
}
