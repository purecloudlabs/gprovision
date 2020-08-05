// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

//Package stream contains chainable functions acting on sizeable data streams
//such as cores. It can write a local copy, compress, and/or upload to s3.
package stream

import (
	"io"
	"os"
	"os/exec"
	fp "path/filepath"

	"github.com/purecloudlabs/gprovision/pkg/corer/opts"
	"github.com/purecloudlabs/gprovision/pkg/log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type NextFn func(cfg *opts.Opts, name string, stream io.Reader) error

func Write(cfg *opts.Opts, suffix string, stream io.Reader) (err error) {
	if cfg.LocalOut != "" {
		var f *os.File
		f, err = os.Create(fp.Join(cfg.LocalOut, suffix))
		if err != nil {
			return
		}
		log.Logf("not using s3 - writing to local file %s", f.Name())
		_, err = io.Copy(f, stream)
		f.Close()
		return
	}
	if cfg.S3bkt == "" {
		log.Logln("no bucket defined, discarding output")
		return
	}
	sess, err := session.NewSession(&aws.Config{
		Region: &cfg.TmplData.Region,
	})
	if err != nil {
		log.Logf("creating aws session: %s", err)
	}
	uploader := s3manager.NewUploader(sess)
	uploader.Concurrency = 1 //doubt this can benefit from concurrency since it reads from stdout

	key := fp.Join(cfg.S3prefix, suffix)
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(cfg.S3bkt),
		Key:    aws.String(key),
		Body:   stream,
	})
	if err != nil {
		log.Logln("failed to upload file:", err, result)
	} else if cfg.Verbose {
		log.Logln("uploaded to", cfg.S3bkt, key)
	}
	return err
}

//compress, pass output to 'nextFn' (typically Upload)
func Compress(opts *opts.Opts, uncompressed string, nextFn NextFn) (err error) {
	upName := fp.Base(uncompressed)
	if opts.CompressExt != "" {
		upName += "." + opts.CompressExt
	}
	in, err := os.Open(uncompressed)
	if err != nil {
		return err
	}
	defer in.Close()
	var stream io.Reader
	if opts.Compresser != "" {
		cmd := exec.Command(opts.Compresser, opts.CompressionLevel)
		cmd.Stdin = in
		ur := &unseekableReader{}
		ur.rdr, err = cmd.StdoutPipe()
		if err != nil {
			return err
		}
		stream = ur
		err = cmd.Start()
		if err != nil {
			return err
		}
		defer func() {
			err = cmd.Wait()
			if err != nil {
				log.Logln("compression error:")
			}
		}()
	} else {
		stream = in
	}
	return nextFn(opts, upName, stream)
}

/*S3 upload does introspection and will find the Seek() method if it exists.
While pipes are files and thus have a seek method, it can't be used. Using it
causes the s3 upload to barf.
To avoid this, create our own type with a Read() method for io.Reader - but no Seek().
*/
type unseekableReader struct {
	rdr io.Reader
}

func (u *unseekableReader) Read(p []byte) (int, error) { return u.rdr.Read(p) }

var _ io.Reader = &unseekableReader{}

//try to write buf to a local file, then upload
// in the unlikely event file creation fails, go straight from the buffer
func LocalCopy(cfg *opts.Opts, localpath string, buf io.Reader) error {
	var stream io.Reader
	f, err := os.Create(localpath)
	if err != nil {
		stream = buf
		log.Logln("creating file:", err)
	} else {
		defer f.Close()
		stream = f
		_, err := io.Copy(f, buf)
		if err != nil {
			log.Logln("writing file:", err)
			stream = buf
		}
		if err == nil {
			_, err = f.Seek(0, io.SeekStart)
			if err != nil {
				log.Logln("seeking:", err)
				f.Close()
				f, err := os.Open(localpath)
				if err != nil {
					log.Logln("re-opening file:", err)
					//extremely unlikely we get here. just give up.
					return err
				}
				stream = f
				defer f.Close()
			}
		}
	}
	return Write(cfg, fp.Base(localpath), stream)
}
