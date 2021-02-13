package utils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"
)

func AddFileToTar(tw *tar.Writer, path, dstPath string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	// now lets create the header as needed for this file within the tarball
	header := new(tar.Header)
	header.Name = dstPath
	header.Size = stat.Size()
	header.Mode = int64(stat.Mode())
	header.ModTime = stat.ModTime()
	// write the header to the tarball archive
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	// copy the file data to the tarball
	if _, err := io.Copy(tw, file); err != nil {
		return err
	}

	return nil
}

func CreateOneFileArchive(w io.Writer, filename string, data []byte) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// add file in archive
	header := new(tar.Header)
	header.Name = filename
	header.Size = int64(len(data))
	header.Mode = int64(0644)
	header.ModTime = time.Now()
	// write the header to the tarball archive
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	// copy the file data to the tarball
	if _, err := io.Copy(tw, bytes.NewReader(data)); err != nil {
		return err
	}

	return nil
}

func CreateArchive(sourcePath string, w io.Writer) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			header, err := tar.FileInfoHeader(info, path)
			if err != nil {
				return err
			}
			header.Name = relPath

			return tw.WriteHeader(header)
		} else {
			return AddFileToTar(tw, path, relPath)
		}
	})
	if err != nil {
		return err
	}

	return nil
}

func OpenArchive(dstPath string, r io.Reader) error {
	// check the format of data (tar.gz)
	gzf, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzf.Close()

	// extract project in the destination folder
	tarReader := tar.NewReader(gzf)
	for true {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("Archive is not a valid tar: %w", err)
		}

		name := header.Name

		switch header.Typeflag {
		case tar.TypeDir: // = directory
			target := path.Join(dstPath, name)
			if _, err := os.Stat(target); os.IsNotExist(err) {
				if err := os.Mkdir(target, 0755); err != nil {
					return fmt.Errorf("Unable to create folder '%s': %w", target, err)
				}
			}
		case tar.TypeReg: // = regular file
			outFile, err := os.Create(path.Join(dstPath, header.Name))
			if err != nil {
				return fmt.Errorf("Unable to create file '%s': %w", name, err)
			}
			defer outFile.Close()

			if _, err := io.Copy(outFile, tarReader); err != nil {
				return fmt.Errorf("Unable to copy file '%s': %w", name, err)
			}
		default:
			msg := fmt.Sprintf("%s : %c %s %s\n",
				"Error when opening archive - unable to figure out type",
				header.Typeflag,
				"in file",
				name,
			)
			return fmt.Errorf(msg)
		}
	}

	return nil
}
