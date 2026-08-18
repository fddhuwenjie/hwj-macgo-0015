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
	file *os.File
	mu   sync.Mutex
	path string
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

// Replay 重放日志，返回所有完整落盘的记录。
//
// 若日志尾部存在因崩溃产生的"撕裂写"——长度头不完整，或负载未写完——
// 仅截断这条不完整记录，保留此前已完整落盘的前缀；随后追加的记录可在
// 截断点续写并再次重放。只有非撕裂的真正 I/O 错误才作为错误返回。
func (j *Journal) Replay() ([][]byte, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, err := j.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	var records [][]byte
	// lastGood 为已完整读入的最后一条记录的结束位置，即下一条记录的起点。
	lastGood := int64(0)
	lenBuf := make([]byte, 4)
	for {
		// 读取 4 字节长度头。0 字节表示正常结束；不足 4 字节表示头部撕裂。
		n, err := io.ReadFull(j.file, lenBuf)
		if err == io.EOF || n == 0 {
			break
		}
		if err == io.ErrUnexpectedEOF {
			// 长度头不完整：丢弃该撕裂尾部，截断到上一条完整记录末尾。
			return records, j.truncateTail(lastGood)
		}
		if err != nil {
			return nil, err
		}

		length := binary.BigEndian.Uint32(lenBuf)
		recStart := lastGood // 本条记录（含长度头）的起始偏移
		data := make([]byte, length)
		if _, err := io.ReadFull(j.file, data); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				// 负载不完整：截断到本条记录起点，整条丢弃。
				return records, j.truncateTail(recStart)
			}
			return nil, err
		}
		records = append(records, data)
		lastGood = recStart + int64(4) + int64(length)
	}
	return records, nil
}

// truncateTail 把日志截断到最近一条完整记录的末尾并回拨读写位置，
// 使后续 O_APPEND 追加与再次 Replay 都从该点开始。
func (j *Journal) truncateTail(offset int64) error {
	if err := j.file.Truncate(offset); err != nil {
		return err
	}
	_, err := j.file.Seek(offset, io.SeekStart)
	return err
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
