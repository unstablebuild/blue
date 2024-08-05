// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package upspin

import (
	stdErrors "errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/retry"
	"upspin.io/errors"
	_ "upspin.io/pack/plain"
	"upspin.io/upspin"
)

func create(t *testing.T, name upspin.PathName) *File {
	f, err := Open(&dummyClient{}, name, os.O_RDWR|os.O_CREATE)
	require.NoError(t, err)
	return f
}

func TestSync(t *testing.T) {
	t.Run("strong consistency is ok", func(t *testing.T) {
		client := &dummyClient{}
		client.returnLookup = func() (*upspin.DirEntry, error) {
			return &upspin.DirEntry{Sequence: 10}, nil
		}

		client.returnPut = func() (*upspin.DirEntry, error) {
			return &upspin.DirEntry{Sequence: 10}, nil
		}

		f, err := Open(client, "a", os.O_RDWR|os.O_CREATE)
		require.NoError(t, err)

		_, err = f.Sync()
		require.NoError(t, err)
	})

	t.Run("versions eventual consistency", func(t *testing.T) {
		client := &dummyClient{}
		var n int64
		client.returnLookup = func() (*upspin.DirEntry, error) {
			n++
			return &upspin.DirEntry{Sequence: n}, nil
		}

		client.returnPut = func() (*upspin.DirEntry, error) {
			return &upspin.DirEntry{Sequence: 10}, nil
		}

		f, err := Open(client, "a", os.O_RDWR|os.O_CREATE)
		require.NoError(t, err)

		_, err = f.Sync()
		require.NoError(t, err)
	})

	t.Run("create eventual consistency", func(t *testing.T) {
		client := &dummyClient{}
		var n int64
		client.returnLookup = func() (*upspin.DirEntry, error) {
			n++
			if n < 5 {
				return nil, errors.E("test", errors.NotExist)
			}
			return &upspin.DirEntry{Sequence: 5}, nil
		}

		client.returnPut = func() (*upspin.DirEntry, error) {
			return &upspin.DirEntry{Sequence: 5}, nil
		}

		f, err := Open(client, "a", os.O_RDWR|os.O_CREATE)
		require.NoError(t, err)

		_, err = f.Sync()
		require.NoError(t, err)
	})

	t.Run("bubbles up first Put error", func(t *testing.T) {
		client := &dummyClient{}
		var i int
		client.returnPut = func() (*upspin.DirEntry, error) {
			i++
			if i != 1 {
				t.Fail()
			}
			return nil, stdErrors.New("$HIMS")
		}

		f, err := Open(client, "a", os.O_RDWR|os.O_CREATE)
		require.NoError(t, err)

		_, err = f.Sync()
		require.Error(t, err)
	})

	t.Run("bubbles up exhaustion of retries", func(t *testing.T) {
		// only allow 2 tries
		defaultSyncStrategy := syncStrategy
		syncStrategy = retry.LimitStrategy(2)
		defer func() {
			syncStrategy = defaultSyncStrategy
		}()

		client := &dummyClient{}
		var n int64
		client.returnLookup = func() (*upspin.DirEntry, error) {
			n++
			return &upspin.DirEntry{Sequence: n}, nil
		}

		client.returnPut = func() (*upspin.DirEntry, error) {
			return &upspin.DirEntry{Sequence: 3}, nil
		}

		f, err := Open(client, "a", os.O_RDWR|os.O_CREATE)
		require.NoError(t, err)

		_, err = f.Sync()
		require.Error(t, err)
	})
}

func TestWrite(t *testing.T) {
	const dummyData = "This is some dummy data."
	fileName := upspin.PathName("foo@bar.com/hello.txt")

	suite := []struct {
		desc   string
		method func(*File) error
	}{
		{"Close", (*File).Close},
		{"Sync", func(f *File) error {
			_, err := f.Sync()
			return err
		}},
		{"Truncate+Write+Sync", func(f *File) error {
			if err := f.Truncate(4); err != nil {
				return err
			}
			if _, err := f.Write([]byte(dummyData[4:])); err != nil {
				return err
			}

			_, err := f.Sync()
			return err
		}},
	}

	for _, test := range suite {
		t.Run(test.desc, func(t *testing.T) {
			f := create(t, fileName)
			n, err := f.Write([]byte(dummyData))
			require.NoError(t, err)
			assert.Equal(t, len(dummyData), n)

			err = test.method(f)
			require.NoError(t, err)

			dummyClient := f.client.(*dummyClient)
			assert.Equal(t, string(dummyClient.putData), dummyData)
		})
	}
}

// copied from upspin.File tests
func TestFileOverflow(t *testing.T) {
	maxInt = 100
	defer func() { maxInt = int64(^uint(0) >> 1) }()
	const (
		user     = "ernest@unstable.build"
		fileName = user + "/" + "file"
	)
	f := create(t, fileName)
	defer f.Close()
	buf := make([]byte, maxInt)
	n, err := f.Write(buf)
	if err != nil {
		t.Fatal("write file:", err)
	}
	if n != int(maxInt) {
		t.Fatalf("write file: expected %d got %d", maxInt, n)
	}
	_, err = f.Write(make([]byte, maxInt))
	if err == nil {
		t.Fatal("write file: expected overflow")
	}
	if !strings.Contains(err.Error(), "file too long") {
		t.Fatal("write file: expected overflow error, got", err)
	}

	n64, err := f.Seek(0, 0)
	if err != nil {
		t.Fatal("seek file:", err)
	}
	if n64 != 0 {
		t.Fatalf("seek begin file: expected 0 got %d", n64)
	}
	n64, err = f.Seek(maxInt, 0)
	if err != nil {
		t.Fatal("seek end file:", err)
	}
	if n64 != maxInt {
		t.Fatalf("seek file: expected %d got %d", maxInt, n64)
	}
	_, err = f.Seek(maxInt+1, 0)
	if err == nil {
		t.Fatal("seek past file: expected error")
	}

	f = create(t, fileName+"x")
	defer f.Close()
	n64, err = f.Seek(maxInt, 0)
	if err != nil {
		t.Fatal("seek maxInt filex:", err)
	}
	if n64 != maxInt {
		t.Fatalf("seek filex: expected %d got %d", maxInt, n64)
	}
	_, err = f.Seek(maxInt+1, 0)
	if err == nil {
		t.Fatal("seek maxint+1 filex: expected error")
	}
}

func TestReadWritable(t *testing.T) {
	t.Run("Read initial content", func(t *testing.T) {
		f, client := makeReadWritableFile(t, "a", "found it!")
		expectFileContent(t, f, client, "found it!")
	})

	t.Run("File open for create", func(t *testing.T) {
		client := &dummyClient{putData: []byte{}}
		f, err := Open(client, "b", os.O_CREATE)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		f, err = Open(client, "b", 0)
		require.NoError(t, err)
		require.NoError(t, f.Close())
	})
	t.Run("O_EXCL", func(t *testing.T) {
		client := &dummyClient{putData: []byte{}}
		f, err := Open(client, "b", os.O_CREATE)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		_, err = Open(client, "b", os.O_CREATE|os.O_EXCL)
		require.Error(t, err)
	})

	t.Run("Truncate initial content", func(t *testing.T) {
		f, client := makeReadWritableFile(t, "a", "breachez")
		err := f.Truncate(0)
		require.NoError(t, err)
		expectFileContent(t, f, client, "")
	})

	t.Run("Write to initial content", func(t *testing.T) {
		f, client := makeReadWritableFile(t, "a", "found it!")
		_, err := f.Write([]byte("hello world"))
		require.NoError(t, err)
		expectFileContent(t, f, client, "found it!hello world")
	})

	t.Run("Write to empty file", func(t *testing.T) {
		f, client := makeReadWritableFile(t, "a", "")
		_, err := f.Write([]byte("hello world"))
		require.NoError(t, err)
		expectFileContent(t, f, client, "hello world")
	})

	t.Run("read from empty file", func(t *testing.T) {
		f, client := makeReadWritableFile(t, "a", "")
		expectFileContent(t, f, client, "")
	})

	t.Run("Seek then write", func(t *testing.T) {
		f, client := makeReadWritableFile(t, "a", "a\nb\n")

		_, err := f.Seek(1, io.SeekStart)
		require.NoError(t, err)

		_, err = f.Write([]byte("hello world"))
		require.NoError(t, err)

		expectFileContent(t, f, client, "ahello world")
	})

	t.Run("Seek then read", func(t *testing.T) {
		f, client := makeReadWritableFile(t, "a", "hello world")
		_, err := f.Seek(1, io.SeekStart)
		require.NoError(t, err)

		got, err := io.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, "ello world", string(got))

		expectFileContent(t, f, client, "hello world")
	})

	t.Run("write then read", func(t *testing.T) {
		f, client := makeReadWritableFile(t, "a", "")
		for j := 0; j < 10; j++ {

			// reset read-side offsets
			_, err := f.Seek(0, io.SeekStart)
			require.NoError(t, err)

			for i := 0; i < 10; i++ {
				n, err := f.Write([]byte(fmt.Sprintf("%d", i)))
				require.NoError(t, err)
				assert.Equal(t, 1, n)

				got, err := io.ReadAll(f)
				require.NoError(t, err)
				assert.Equal(t, fmt.Sprintf("%d", i), string(got))
			}

			expectFileContent(t, f, client, "0123456789")

			err = f.Truncate(0)
			require.NoError(t, err)
		}
	})
}

func expectFileContent(t *testing.T, f *File, client *dummyClient, expected string) {
	_, err := f.Seek(0, io.SeekStart)
	require.NoError(t, err)

	got, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, expected, string(got))

	_, err = f.Sync()
	require.NoError(t, err)
	assert.Equal(t, expected, string(client.putData))
}

func makeReadWritableFile(t *testing.T, name upspin.PathName, content string) (*File, *dummyClient) {
	mockClient := &dummyClient{putData: []byte(content)}
	f, err := Open(mockClient, name, os.O_CREATE|os.O_RDWR)
	require.NoError(t, err)
	if err != nil {
		t.Fatalf("unexpected ReadWritable error: %v", err)
	}
	return f, mockClient
}

type dummyClient struct {
	putData      []byte
	returnLookup func() (*upspin.DirEntry, error)
	returnPut    func() (*upspin.DirEntry, error)
}

var _ upspin.Client = (*dummyClient)(nil)

func (d *dummyClient) Get(name upspin.PathName) ([]byte, error) {
	return d.putData, nil
}
func (d *dummyClient) Lookup(name upspin.PathName, followFinal bool) (*upspin.DirEntry, error) {
	if d.returnLookup != nil {
		return d.returnLookup()
	}
	return new(upspin.DirEntry), nil
}
func (d *dummyClient) Put(name upspin.PathName, data []byte) (*upspin.DirEntry, error) {
	d.putData = make([]byte, len(data))
	copy(d.putData, data)
	if d.returnPut != nil {
		return d.returnPut()
	}
	return new(upspin.DirEntry), nil
}
func (d *dummyClient) PutSequenced(name upspin.PathName, seq int64, data []byte) (*upspin.DirEntry, error) {
	d.putData = make([]byte, len(data))
	copy(d.putData, data)
	return new(upspin.DirEntry), nil
}
func (d *dummyClient) PutLink(oldName, newName upspin.PathName) (*upspin.DirEntry, error) {
	return nil, nil
}
func (d *dummyClient) PutDuplicate(oldName, newName upspin.PathName) (*upspin.DirEntry, error) {
	return nil, nil
}
func (d *dummyClient) MakeDirectory(dirName upspin.PathName) (*upspin.DirEntry, error) {
	return nil, nil
}
func (d *dummyClient) Delete(name upspin.PathName) error {
	return nil
}
func (d *dummyClient) Glob(pattern string) ([]*upspin.DirEntry, error) {
	return nil, nil
}
func (d *dummyClient) Create(name upspin.PathName) (upspin.File, error) {
	return nil, nil
}
func (d *dummyClient) Open(name upspin.PathName) (upspin.File, error) {
	return nil, nil
}
func (d *dummyClient) DirServer(name upspin.PathName) (upspin.DirServer, error) {
	return nil, nil
}
func (d *dummyClient) Rename(oldName, newName upspin.PathName) (*upspin.DirEntry, error) {
	return nil, nil
}
func (d *dummyClient) SetTime(name upspin.PathName, t upspin.Time) error {
	return nil
}
