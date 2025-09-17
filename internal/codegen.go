package internal

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/noders-team/go-daml/internal/model"
	"github.com/rs/zerolog/log"
)

func UnzipDar(src string, output *string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			log.Err(err).Msgf("failed to close zip file")
		}
	}()

	if output == nil {
		tmpDir := os.TempDir()
		output = &tmpDir
	}

	os.MkdirAll(*output, 0o755)

	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(*output, f.Name)

		if !strings.HasPrefix(path, filepath.Clean(*output)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, 0o755)
		} else {
			os.MkdirAll(filepath.Dir(path), 0o755)
			outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			defer func() {
				if err := outFile.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(outFile, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func GetManifest(srcPath string) (*model.Manifest, error) {
	manifestPath := strings.Join([]string{srcPath, "META-INF", "MANIFEST.MF"}, "/")
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	content := strings.ReplaceAll(string(b), "\n ", "")

	manifest := &model.Manifest{}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Manifest-Version:") {
			manifest.Version = strings.TrimSpace(strings.TrimPrefix(line, "Manifest-Version:"))
		} else if strings.HasPrefix(line, "Created-By:") {
			manifest.CreatedBy = strings.TrimSpace(strings.TrimPrefix(line, "Created-By:"))
		} else if strings.HasPrefix(line, "Name:") {
			manifest.Name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
		} else if strings.HasPrefix(line, "Sdk-Version:") {
			manifest.SdkVersion = strings.TrimSpace(strings.TrimPrefix(line, "Sdk-Version:"))
		} else if strings.HasPrefix(line, "Main-Dalf:") {
			manifest.MainDalf = strings.TrimSpace(strings.TrimPrefix(line, "Main-Dalf:"))
		} else if strings.HasPrefix(line, "Dalfs:") {
			dalfsStr := strings.TrimSpace(strings.TrimPrefix(line, "Dalfs:"))
			if dalfsStr != "" {
				manifest.Dalfs = strings.Split(dalfsStr, ", ")
				for i, dalf := range manifest.Dalfs {
					manifest.Dalfs[i] = strings.TrimSpace(dalf)
				}
			}
		} else if strings.HasPrefix(line, "Format:") {
			manifest.Format = strings.TrimSpace(strings.TrimPrefix(line, "Format:"))
		} else if strings.HasPrefix(line, "Encryption:") {
			manifest.Encryption = strings.TrimSpace(strings.TrimPrefix(line, "Encryption:"))
		}
	}

	if manifest.MainDalf == "" {
		return nil, fmt.Errorf("main-dalf not found in manifest")
	}

	return manifest, nil
}
