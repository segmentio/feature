package feature

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type CollectionIter struct{ dir }

func (it *CollectionIter) Close() error { return it.close() }

func (it *CollectionIter) Next() bool { return it.next() }

func (it *CollectionIter) Name() string { return it.name() }

func (it *CollectionIter) IDs() *IDIter {
	return &IDIter{readfile(filepath.Join(it.path, it.name()))}
}

type IDIter struct{ file }

func (it *IDIter) Close() error { return it.close() }

func (it *IDIter) Next() bool { return it.next() }

func (it *IDIter) Name() string { return it.line }

type Collection struct {
	file *os.File
	buf  *bufio.Writer
	err  error
}

func (col *Collection) Path() string {
	if col.file != nil {
		return col.file.Name()
	}
	return ""
}

func (col *Collection) Close() error {
	if col.buf != nil {
		col.err = col.buf.Flush()
		col.buf = nil
	}
	if col.file != nil {
		col.file.Close()
		col.file = nil
	}
	return col.err
}

func (col *Collection) Sync() error {
	if col.buf != nil {
		return col.buf.Flush()
	}
	return nil
}

func (col *Collection) IDs() *IDIter {
	return &IDIter{col.ids()}
}

func (col *Collection) ids() file {
	if col.file != nil {
		return readfile(col.file.Name())
	}
	return file{}
}

func (col *Collection) Add(id string) error {
	if col.err == nil {
		col.err = col.writeLine(id)
	}
	return col.err
}

func (col *Collection) writeLine(s string) error {
	if col.file == nil {
		return nil
	}
	if col.buf == nil {
		col.buf = bufio.NewWriter(col.file)
	}
	if _, err := col.buf.WriteString(s); err != nil {
		return err
	}
	return col.buf.WriteByte('\n')
}

type file struct {
	file *os.File
	buf  *bufio.Reader
	line string
	err  error
}

func (f *file) close() error {
	if f.file != nil {
		f.file.Close()
	}
	f.file, f.buf, f.line = nil, nil, ""
	return f.err
}

func (f *file) next() bool {
	if f.file == nil {
		return false
	}

	if f.buf == nil {
		f.buf = bufio.NewReader(f.file)
	}

	for {
		line, err := f.buf.ReadString('\n')

		if err != nil && (err != io.EOF || line == "") {
			if err == io.EOF {
				err = nil
			}
			f.err = err
			f.close()
			return false
		}

		if line = strings.TrimSuffix(line, "\n"); line != "" {
			f.line = line
			return true
		}
	}
}

func readfile(path string) file {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
	}
	return file{file: f, err: err}
}
