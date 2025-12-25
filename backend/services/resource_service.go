package services

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type ResourceService struct{}

func NewResourceService() *ResourceService {
	return &ResourceService{}
}

// PackResources 打包图片资源为 zip 文件
func (s *ResourceService) PackResources(iconsDir, bigPicsDir, outputPath string) error {
	// 创建 zip 文件
	zipFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 添加 icons 目录
	if err := s.addDirToZip(zipWriter, iconsDir, "icons"); err != nil {
		return fmt.Errorf("failed to add icons: %w", err)
	}

	// 添加 bigpics 目录
	if err := s.addDirToZip(zipWriter, bigPicsDir, "bigpics"); err != nil {
		return fmt.Errorf("failed to add bigpics: %w", err)
	}

	return nil
}

// addDirToZip 将目录添加到 zip
func (s *ResourceService) addDirToZip(zipWriter *zip.Writer, sourceDir, zipPath string) error {
	// 检查目录是否存在
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		// 目录不存在，创建空目录条目
		_, err := zipWriter.Create(zipPath + "/")
		return err
	}

	// 遍历目录
	return filepath.Walk(sourceDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录本身
		if info.IsDir() {
			return nil
		}

		// 计算在 zip 中的相对路径
		relPath, err := filepath.Rel(sourceDir, filePath)
		if err != nil {
			return err
		}
		zipFilePath := filepath.Join(zipPath, relPath)

		// 创建 zip 文件条目
		writer, err := zipWriter.Create(zipFilePath)
		if err != nil {
			return err
		}

		// 读取并写入文件内容
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

// GetFileSize 获取文件大小
func (s *ResourceService) GetFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}


