package main

import (
	"os"
	"path/filepath"
	"strconv"
)

type OutputCollector struct {
	path                 string
	prefix               string
	extension            string
	files                []string
	latestFileCharacters int
	maxFileChars         int
	latestFileRef        *os.File
}

func NewOutputCollector(path, prefix, extension string, maxFileChars int) (*OutputCollector, error) {
	firstFile := filepath.Join(path, prefix+"-0."+extension)
	file, err := os.OpenFile(firstFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}

	return &OutputCollector{
		path:                 path,
		prefix:               prefix,
		extension:            extension,
		files:                []string{firstFile},
		latestFileCharacters: 0,
		maxFileChars:         maxFileChars,
		latestFileRef:        file,
	}, nil
}

func (oc *OutputCollector) WriteString(header string, data string) error {
	if oc.latestFileCharacters+len(data) > oc.maxFileChars {
		if err := oc.createNewFile(header); err != nil {
			return err
		}
	}

	_, err := oc.latestFileRef.WriteString(data)
	if err != nil {
		return err
	}

	oc.latestFileCharacters += len(data)
	return nil
}

func (oc *OutputCollector) createNewFile(header string) error {
	newFileName := filepath.Join(oc.path, oc.prefix+"-"+strconv.Itoa(len(oc.files))+"."+oc.extension)
	file, err := os.OpenFile(newFileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	oc.latestFileRef.Close()
	oc.latestFileRef = file
	oc.latestFileCharacters = 0
	oc.files = append(oc.files, newFileName)

	_, err = file.WriteString(header)
	if err != nil {
		return err
	}
	oc.latestFileCharacters += len(header)

	return nil
}

func (oc *OutputCollector) GetFiles() []string {
	return oc.files
}

func (oc *OutputCollector) Close() error {
	return oc.latestFileRef.Close()
}
