//go:build !linux

package safepath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func canonicalRoot(rootDir string) (string, error) {
	if rootDir == "" {
		return "", errors.New("raiz autorizada não pode ser vazia")
	}

	cleanRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", fmt.Errorf("falha ao resolver raiz autorizada: %w", err)
	}

	realRoot, err := filepath.EvalSymlinks(cleanRoot)
	if err != nil {
		realRoot = cleanRoot
	}
	return filepath.Clean(realRoot), nil
}

func sanitizeRelPath(realRoot, reqPath string) (string, string, error) {
	cleanReq := filepath.Clean(reqPath)
	if cleanReq == "" || cleanReq == "." {
		return realRoot, ".", nil
	}

	var combined string
	if filepath.IsAbs(cleanReq) {
		combined = cleanReq
	} else {
		combined = filepath.Join(realRoot, cleanReq)
	}
	combined = filepath.Clean(combined)

	relToRoot, err := filepath.Rel(realRoot, combined)
	if err != nil || strings.HasPrefix(relToRoot, "..") || relToRoot == ".." {
		return "", "", ErrPathTraversal
	}

	// Resolução física componente a componente contra symlink traversal intermediário
	parts := strings.Split(relToRoot, string(filepath.Separator))
	current := realRoot

	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", "", ErrPathTraversal
		}

		next := filepath.Join(current, part)
		fi, err := os.Lstat(next)
		if err != nil {
			if os.IsNotExist(err) {
				current = next
				continue
			}
			return "", "", err
		}

		if fi.Mode()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(next)
			if err != nil {
				return "", "", fmt.Errorf("falha ao resolver link simbólico: %w", err)
			}
			targetClean := filepath.Clean(target)
			relSym, errRel := filepath.Rel(realRoot, targetClean)
			if errRel != nil || strings.HasPrefix(relSym, "..") || relSym == ".." {
				return "", "", ErrSymlinkEscape
			}
			current = targetClean
		} else {
			current = next
		}

		relCheck, errCheck := filepath.Rel(realRoot, current)
		if errCheck != nil || strings.HasPrefix(relCheck, "..") || relCheck == ".." {
			return "", "", ErrPathTraversal
		}
	}

	return current, relToRoot, nil
}

func validateDescriptor(f *os.File) error {
	fi, err := f.Stat()
	if err != nil {
		return err
	}

	// Rejeita FIFOs, sockets, pipes e dispositivos
	mode := fi.Mode()
	if mode&os.ModeNamedPipe != 0 || mode&os.ModeSocket != 0 || mode&os.ModeDevice != 0 || mode&os.ModeCharDevice != 0 {
		return ErrNotRegularFile
	}

	return nil
}

func openBeneathInternal(rootDir string, reqPath string) (*os.File, string, error) {
	realRoot, err := canonicalRoot(rootDir)
	if err != nil {
		return nil, "", err
	}

	resolved, _, err := sanitizeRelPath(realRoot, reqPath)
	if err != nil {
		return nil, "", err
	}

	// #nosec G304 -- caminho validado rigorosamente por sanitizeRelPath e restrito à raiz
	f, err := os.OpenFile(resolved, os.O_RDONLY, 0)
	if err != nil {
		return nil, "", err
	}

	if err := validateDescriptor(f); err != nil {
		_ = f.Close()
		return nil, "", err
	}

	return f, resolved, nil
}

func writeBeneathInternal(rootDir string, reqPath string, data []byte, perm os.FileMode) (string, error) {
	realRoot, err := canonicalRoot(rootDir)
	if err != nil {
		return "", err
	}

	resolved, _, err := sanitizeRelPath(realRoot, reqPath)
	if err != nil {
		return "", err
	}

	parentDir := filepath.Dir(resolved)
	if err := os.MkdirAll(parentDir, 0750); err != nil {
		return "", err
	}

	if perm == 0 {
		perm = 0644
	}

	// #nosec G304 -- caminho validado rigorosamente por sanitizeRelPath contra directory traversal
	f, err := os.OpenFile(resolved, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := validateDescriptor(f); err != nil {
		return "", err
	}

	fi, err := f.Stat()
	if err == nil && fi.IsDir() {
		return "", ErrIsDirectory
	}

	if _, err := f.Write(data); err != nil {
		return "", err
	}

	return resolved, nil
}

func createBeneathInternal(rootDir string, reqPath string, perm os.FileMode) (string, error) {
	realRoot, err := canonicalRoot(rootDir)
	if err != nil {
		return "", err
	}

	resolved, _, err := sanitizeRelPath(realRoot, reqPath)
	if err != nil {
		return "", err
	}

	parentDir := filepath.Dir(resolved)
	if err := os.MkdirAll(parentDir, 0750); err != nil {
		return "", err
	}

	if perm == 0 {
		perm = 0644
	}

	// #nosec G304 -- caminho validado rigorosamente por sanitizeRelPath contra directory traversal
	f, err := os.OpenFile(resolved, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return "", err
	}
	_ = f.Close()

	return resolved, nil
}

func mkdirBeneathInternal(rootDir string, reqPath string, perm os.FileMode) (string, error) {
	realRoot, err := canonicalRoot(rootDir)
	if err != nil {
		return "", err
	}

	resolved, _, err := sanitizeRelPath(realRoot, reqPath)
	if err != nil {
		return "", err
	}

	if perm == 0 {
		perm = 0750
	}

	if err := os.MkdirAll(resolved, perm); err != nil {
		return "", err
	}

	return resolved, nil
}

func renameBeneathInternal(rootDir string, oldPath string, newPath string) error {
	realRoot, err := canonicalRoot(rootDir)
	if err != nil {
		return err
	}

	resolvedOld, _, err := sanitizeRelPath(realRoot, oldPath)
	if err != nil {
		return fmt.Errorf("caminho de origem inválido: %w", err)
	}

	resolvedNew, _, err := sanitizeRelPath(realRoot, newPath)
	if err != nil {
		return fmt.Errorf("caminho de destino inválido: %w", err)
	}

	newParent := filepath.Dir(resolvedNew)
	if err := os.MkdirAll(newParent, 0750); err != nil {
		return fmt.Errorf("diretório pai de destino inválido: %w", err)
	}

	return os.Rename(resolvedOld, resolvedNew)
}

func removeBeneathInternal(rootDir string, reqPath string) error {
	realRoot, err := canonicalRoot(rootDir)
	if err != nil {
		return err
	}

	resolved, rel, err := sanitizeRelPath(realRoot, reqPath)
	if err != nil {
		return err
	}

	if rel == "." || rel == "" || resolved == realRoot {
		return errors.New("proibido excluir o diretório raiz")
	}

	return os.RemoveAll(resolved)
}

func listBeneathInternal(rootDir string, reqPath string) ([]FileEntry, string, error) {
	realRoot, err := canonicalRoot(rootDir)
	if err != nil {
		return nil, "", err
	}

	resolved, _, err := sanitizeRelPath(realRoot, reqPath)
	if err != nil {
		return nil, "", err
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, "", err
	}

	items := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		itemPath := filepath.Join(resolved, e.Name())
		items = append(items, FileEntry{
			Name:       e.Name(),
			Path:       itemPath,
			IsDir:      e.IsDir(),
			Size:       info.Size(),
			Mode:       info.Mode().String(),
			ModifiedAt: info.ModTime(),
		})
	}

	return items, resolved, nil
}
