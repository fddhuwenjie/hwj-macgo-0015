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

// TruncateSafe 安全截断到指定偏移，并同步落盘以保证崩溃安全。
func (j *Journal) TruncateSafe(offset int64) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if err := j.file.Truncate(offset); err != nil {
		return err
	}
	return j.file.Sync()
}

// maxRecordLen 单条记录数据上限，超过则视为损坏（避免畸形长度头导致超大分配）。
const maxRecordLen = 16 << 20 // 16 MiB

// Replay 重放日志，返回所有合法记录。
func (j *Journal) Replay() ([][]byte, error) {
	records, _, err := j.ReplayWithOffset()
	return records, err
}

// ReplayWithOffset 重放日志，返回所有合法记录与“合法尾部偏移”。
//
// 合法尾部偏移 validOffset 是已成功读取的记录在文件中的结束字节位置：
//   - 若文件以完整记录正常结束，validOffset == 文件大小；
//   - 若末尾存在崩溃残留的不完整记录，validOffset 指向该残留起点，
//     可由 TruncateCorruptTail 安全截断，而合法记录保持不变。
//
// 末尾不完整记录被视为崩溃残留：停止重放而不报错，以免丢弃前面已读取的合法证据。
// 合法记录一律保留（不再被跳过）；空记录视为非法并被过滤。
func (j *Journal) ReplayWithOffset() (records [][]byte, validOffset int64, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, err := j.file.Seek(0, io.SeekStart); err != nil {
		return nil, 0, err
	}
	lenBuf := make([]byte, 4)
	for {
		n, err := io.ReadFull(j.file, lenBuf)
		if err == io.EOF || n == 0 {
			// 干净结束：合法尾部即当前位置。
			break
		}
		if err == io.ErrUnexpectedEOF {
			// 末尾不完整的长度头：崩溃残留，停止重放，保留已读合法记录。
			break
		}
		if err != nil {
			// 真实读取错误：中止并保留证据，交由上层处理。
			return records, validOffset, err
		}
		length := binary.BigEndian.Uint32(lenBuf)
		if length > maxRecordLen {
			// 长度头异常：视为损坏尾部，停止重放而不做超大分配。
			break
		}
		data := make([]byte, length)
		if _, err := io.ReadFull(j.file, data); err != nil {
			if err == io.ErrUnexpectedEOF {
				// 末尾不完整的数据体：崩溃残留，停止重放。
				break
			}
			return records, validOffset, err
		}
		if !validate(data) {
			// 非法（空）记录：跳过，但不丢弃其后可能存在的合法记录。
			validOffset, _ = j.file.Seek(0, io.SeekCurrent)
			continue
		}
		records = append(records, data)
		validOffset, _ = j.file.Seek(0, io.SeekCurrent)
	}
	return records, validOffset, nil
}

// TruncateCorruptTail 截断日志末尾的“损坏尾部”（崩溃残留的不完整记录）。
//
// 仅当 validOffset 之后存在残留字节时才截断；validOffset 之前的合法记录
// 作为证据完整保留，绝不在成功读取后清空。截断后同步落盘。
func (j *Journal) TruncateCorruptTail(validOffset int64) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	fi, err := j.file.Stat()
	if err != nil {
		return err
	}
	if validOffset >= fi.Size() {
		// 无损坏尾部：保留全部合法记录作为证据，不做任何改动。
		return nil
	}
	if err := j.file.Truncate(validOffset); err != nil {
		return err
	}
	// 回到合法记录末尾，便于后续追加；同步落盘保证崩溃安全。
	if _, err := j.file.Seek(validOffset, io.SeekStart); err != nil {
		return err
	}
	return j.file.Sync()
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

// ValidRecord 验证单条记录是否合法，供重放与恢复调用方统一判据。
// 当前记录格式为「4字节长度 + 数据」，无独立校验位，
// 故以“非空”作为最低合法性判据：空记录视为非法。
// 合法记录返回 true；非法记录返回 false（调用方据此跳过，绝不丢弃合法记录）。
func ValidRecord(data []byte) bool {
	return len(data) > 0
}

// validate 为包内重放使用的别名，保持语义集中。
func validate(data []byte) bool { return ValidRecord(data) }

var _ = fmt.Sprintf
