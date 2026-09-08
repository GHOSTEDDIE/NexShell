package zmodem

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Progress func(name string, transferred, total int64)

// The caller must close its transport on context cancellation to interrupt a blocked Read.
func (c *Codec) Send(ctx context.Context, files []string, initial *Header, progress Progress) (err error) {
	defer func() {
		if err != nil {
			c.Cancel()
		}
	}()
	var h Header
	if initial != nil {
		h = *initial
	} else {
		h, err = c.ReadHeader()
		if err != nil {
			return err
		}
	}
	if h.Type != ZRINIT {
		return fmt.Errorf("expected ZRINIT, got %d", h.Type)
	}
	for _, name := range files {
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = c.sendFile(ctx, name, progress); err != nil {
			return err
		}
	}
	if err = c.WriteHeader(Position(ZFIN, 0)); err != nil {
		return err
	}
	for tries := 0; tries < 10; tries++ {
		h, err = c.ReadHeader()
		if err != nil {
			return err
		}
		if h.Type == ZFIN {
			_, err = io.WriteString(c.Writer, "OO")
			return err
		}
	}
	return errors.New("ZFIN handshake failed")
}
func (c *Codec) sendFile(ctx context.Context, name string, progress Progress) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > math.MaxUint32 {
		return errors.New("ZMODEM requires a regular file up to 4 GiB")
	}
	if err = c.WriteHeader(Position(ZFILE, 0)); err != nil {
		return err
	}
	metadata := []byte(filepath.Base(name) + "\x00" + strconv.FormatInt(info.Size(), 10) + " 0 100600 0 1 " + strconv.FormatInt(info.Size(), 10) + "\x00")
	if err = c.WriteData(metadata, ZCRCW, false); err != nil {
		return err
	}
	var pos uint32
	for tries := 0; tries < 20; tries++ {
		h, e := c.ReadHeader()
		if e != nil {
			return e
		}
		switch h.Type {
		case ZSKIP:
			return nil
		case ZCRC:
			n := int64(h.Position())
			if n == 0 {
				n = info.Size()
			}
			if n > info.Size() {
				return errors.New("invalid CRC range")
			}
			f.Seek(0, io.SeekStart)
			hash := crc32.NewIEEE()
			if _, e = io.CopyN(hash, f, n); e != nil {
				return e
			}
			if e = c.WriteHeader(Position(ZCRC, hash.Sum32())); e != nil {
				return e
			}
			continue
		case ZRPOS:
			pos = h.Position()
		default:
			continue
		}
		break
	}
	if int64(pos) > info.Size() {
		return errors.New("invalid resume offset")
	}
	buf := make([]byte, 8192)
	retries := 0
	for int64(pos) < info.Size() {
		if err = ctx.Err(); err != nil {
			return err
		}
		if _, err = f.Seek(int64(pos), io.SeekStart); err != nil {
			return err
		}
		n, e := f.Read(buf)
		if e != nil && e != io.EOF {
			return e
		}
		if n == 0 {
			return io.ErrUnexpectedEOF
		}
		if err = c.WriteHeader(Position(ZDATA, pos)); err != nil {
			return err
		}
		if err = c.WriteData(buf[:n], ZCRCW, false); err != nil {
			return err
		}
		h, e := c.ReadHeader()
		if e != nil {
			return e
		}
		switch h.Type {
		case ZACK:
			if h.Position() != pos+uint32(n) {
				return errors.New("incorrect acknowledgement position")
			}
			pos += uint32(n)
			retries = 0
			if progress != nil {
				progress(filepath.Base(name), int64(pos), info.Size())
			}
		case ZRPOS:
			if int64(h.Position()) > info.Size() {
				return errors.New("invalid restart offset")
			}
			pos = h.Position()
			retries++
			if retries > 20 {
				return errors.New("too many retries")
			}
		default:
			return fmt.Errorf("unexpected data acknowledgement %d", h.Type)
		}
	}
	if err = c.WriteHeader(Position(ZEOF, pos)); err != nil {
		return err
	}
	h, err := c.ReadHeader()
	if err != nil {
		return err
	}
	if h.Type != ZRINIT {
		return fmt.Errorf("expected next file handshake, got %d", h.Type)
	}
	if progress != nil {
		progress(filepath.Base(name), info.Size(), info.Size())
	}
	return nil
}

func safeName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00") && filepath.Base(name) == name
}
func (c *Codec) Receive(ctx context.Context, directory string, initial *Header, progress Progress) (err error) {
	defer func() {
		if err != nil {
			c.Cancel()
		}
	}()
	if err = os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	ready := Header{Type: ZRINIT, Data: [4]byte{0, 0, 0, 0x23}}
	if err = c.WriteHeader(ready); err != nil {
		return err
	}
	var file *os.File
	var name string
	var size, position int64
	defer func() {
		if file != nil {
			file.Close()
		}
	}()
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		h, e := c.ReadHeader()
		if e != nil {
			return e
		}
		switch h.Type {
		case ZRQINIT:
			if err = c.WriteHeader(ready); err != nil {
				return err
			}
		case ZSINIT:
			if _, _, err = c.ReadData(h.CRC32); err != nil {
				return err
			}
			if err = c.WriteHeader(Position(ZACK, 1)); err != nil {
				return err
			}
		case ZFILE:
			meta, _, e := c.ReadData(h.CRC32)
			if e != nil {
				return e
			}
			parts := strings.SplitN(string(meta), "\x00", 2)
			name = parts[0]
			if !safeName(name) || len(parts) < 2 {
				return errors.New("unsafe or invalid received file name")
			}
			fields := strings.Fields(strings.TrimRight(parts[1], "\x00"))
			if len(fields) == 0 {
				return errors.New("missing file size")
			}
			size, e = strconv.ParseInt(fields[0], 10, 64)
			if e != nil || size < 0 || size > math.MaxUint32 {
				return errors.New("invalid file size")
			}
			p := filepath.Join(directory, name)
			position = 0
			if info, e := os.Lstat(p); e == nil {
				if !info.Mode().IsRegular() || info.Size() > size {
					return errors.New("existing destination is incompatible")
				}
				position = info.Size()
				if position > 0 {
					existing, e := os.Open(p)
					if e != nil {
						return e
					}
					hash := crc32.NewIEEE()
					_, e = io.Copy(hash, existing)
					existing.Close()
					if e != nil {
						return e
					}
					if e = c.WriteHeader(Position(ZCRC, uint32(position))); e != nil {
						return e
					}
					answer, e := c.ReadHeader()
					if e != nil {
						return e
					}
					if answer.Type != ZCRC || answer.Position() != hash.Sum32() {
						return errors.New("existing file prefix differs; resume refused")
					}
				}
			} else if !os.IsNotExist(e) {
				return e
			}
			file, err = os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			if _, err = file.Seek(position, io.SeekStart); err != nil {
				return err
			}
			if err = c.WriteHeader(Position(ZRPOS, uint32(position))); err != nil {
				return err
			}
		case ZDATA:
			if file == nil {
				return errors.New("data without file")
			}
			if int64(h.Position()) != position {
				return errors.New("unexpected data position")
			}
			for {
				data, end, e := c.ReadData(h.CRC32)
				if e != nil {
					if err = c.WriteHeader(Position(ZRPOS, uint32(position))); err != nil {
						return err
					}
					break
				}
				if position+int64(len(data)) > size {
					return errors.New("data exceeds advertised size")
				}
				n, e := file.Write(data)
				if e != nil {
					return e
				}
				position += int64(n)
				if progress != nil {
					progress(name, position, size)
				}
				if end == ZCRCQ || end == ZCRCW {
					if err = c.WriteHeader(Position(ZACK, uint32(position))); err != nil {
						return err
					}
				}
				if end == ZCRCE || end == ZCRCW {
					break
				}
			}
		case ZEOF:
			if file == nil || int64(h.Position()) != position || position != size {
				return errors.New("incomplete file")
			}
			if err = file.Sync(); err != nil {
				return err
			}
			if err = file.Close(); err != nil {
				return err
			}
			file = nil
			if err = c.WriteHeader(ready); err != nil {
				return err
			}
		case ZFIN:
			if file != nil {
				return errors.New("transfer ended before file completion")
			}
			if err = c.WriteHeader(Position(ZFIN, 0)); err != nil {
				return err
			}
			for count := 0; count < 32; count++ {
				b, e := c.raw()
				// Some PTY peers close immediately after the acknowledged ZFIN.
				// All files have already passed CRC and length checks at this point.
				if e == io.EOF {
					return nil
				}
				if e != nil {
					return e
				}
				if b == 'O' {
					next, e := c.raw()
					if e != nil {
						return e
					}
					if next == 'O' {
						return nil
					}
				}
			}
			return errors.New("missing final OO")
		case ZABORT:
			return errors.New("remote aborted transfer")
		default:
			return fmt.Errorf("unexpected receive header %d", h.Type)
		}
	}
}
