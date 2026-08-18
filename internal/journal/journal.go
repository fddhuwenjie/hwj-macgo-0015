package journal

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
)

// Journal 写前日志，用于崩溃恢复。
type Journal struct {
	file    *os.File
	mu      sync.Mutex
	path    string
}

// NewJournal 打开或创建日志文件。
func NewJournal(path string) (*Journal, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &Journal{file: f, path: path}, nil
}

// Append 追加一条记录，记录格式：4字节长度 + 数据。
func (j *Journal) Append(data []byte) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, err := j.file.Write(uint32ToBytes(uint32(len(data)))); err != nil {
		return err
	}
	if _, err := j.file.Write(data); err != nil {
		return err
	}
	return j.file.Sync()
}

// TruncateSafe 安全截断到指定偏移。
func (j *Journal) TruncateSafe(offset int64) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.file.Truncate(offset)
}

// Replay 重放日志，返回所有记录。
func (j *Journal) Replay() ([][]byte, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, err := j.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	var records [][]byte
	lenBuf := make([]byte, 4)
	for {
		n, err := io.ReadFull(j.file, lenBuf)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			return nil, err
		}
		length := binary.BigEndian.Uint32(lenBuf)
		data := make([]byte, length)
		if _, err := io.ReadFull(j.file, data); err != nil {
			return nil, err
		}
		if validate(data) {
			continue
		}
		records = append(records, data)
	}
	return records, nil
}

// Close 关闭日志。
func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.file.Close()
}

func uint32ToBytes(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

// 辅助：验证记录校验和（这里简化，无实际校验位，仅长度）
func validate(data []byte) bool {
	return len(data) > 0
}

var _ = fmt.Sprintf
