//go:build linux

package safepath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func canonicalRoot(rootDir string) (string, int, error) {
	if rootDir == "" {
		return "", -1, errors.New("raiz autorizada não pode ser vazia")
	}

	cleanRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", -1, fmt.Errorf("falha ao resolver raiz autorizada: %w", err)
	}

	realRoot, err := filepath.EvalSymlinks(cleanRoot)
	if err != nil {
		realRoot = cleanRoot
	}
	realRoot = filepath.Clean(realRoot)

	rootFd, err := unix.Open(realRoot, unix.O_DIRECTORY|unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", -1, fmt.Errorf("falha ao abrir descritor de diretório raiz: %w", err)
	}

	return realRoot, rootFd, nil
}

func sanitizeRelPath(realRoot, reqPath string) (string, error) {
	cleanReq := filepath.Clean(reqPath)
	if cleanReq == "" || cleanReq == "." {
		return ".", nil
	}

	// Se for absoluto, extrai o caminho relativo à raiz autorizada
	if filepath.IsAbs(cleanReq) {
		rel, err := filepath.Rel(realRoot, cleanReq)
		if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
			return "", ErrPathTraversal
		}
		cleanReq = rel
	}

	cleanReq = strings.TrimPrefix(cleanReq, string(filepath.Separator))
	cleanReq = strings.TrimPrefix(cleanReq, "/")
	if strings.HasPrefix(cleanReq, "..") || strings.Contains(cleanReq, "/../") || cleanReq == ".." {
		return "", ErrPathTraversal
	}

	return cleanReq, nil
}

func openat2OrSecureWalk(rootFd int, relPath string, flags int, perm os.FileMode) (int, error) {
	// #nosec G115 -- flags e perm são inteiros canônicos de abertura de arquivos
	how := unix.OpenHow{
		Flags:   uint64(flags | unix.O_CLOEXEC),
		Mode:    uint64(perm),
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS,
	}

	fd, err := unix.Openat2(rootFd, relPath, &how)
	if err == nil {
		return fd, nil
	}

	if errors.Is(err, unix.EXDEV) {
		return -1, ErrPathTraversal
	}
	if errors.Is(err, unix.ELOOP) {
		return -1, ErrSymlinkEscape
	}

	// Se openat2 não for suportado pelo kernel (retorna ENOSYS), usa resolução estrita por descritores intermediários com O_NOFOLLOW
	if errors.Is(err, unix.ENOSYS) {
		return secureOpenatWalk(rootFd, relPath, flags, perm)
	}

	return -1, err
}

func secureOpenatWalk(rootFd int, relPath string, flags int, perm os.FileMode) (int, error) {
	parts := strings.Split(relPath, "/")
	currFd := rootFd
	needClose := false

	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if needClose {
				_ = unix.Close(currFd)
			}
			return -1, ErrPathTraversal
		}

		isLast := (i == len(parts)-1)
		if isLast {
			fd, err := unix.Openat(currFd, part, flags|unix.O_CLOEXEC|unix.O_NOFOLLOW, uint32(perm))
			if needClose {
				_ = unix.Close(currFd)
			}
			if err != nil {
				if errors.Is(err, unix.ELOOP) {
					return -1, ErrSymlinkEscape
				}
				return -1, err
			}
			return fd, nil
		}

		nextFd, err := unix.Openat(currFd, part, unix.O_DIRECTORY|unix.O_PATH|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if needClose {
			_ = unix.Close(currFd)
		}
		if err != nil {
			if errors.Is(err, unix.ELOOP) {
				return -1, ErrSymlinkEscape
			}
			return -1, err
		}
		currFd = nextFd
		needClose = true
	}

	if needClose {
		_ = unix.Close(currFd)
	}
	return -1, errors.New("caminho vazio")
}

func validateDescriptor(fd int) error {
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return err
	}

	fileType := stat.Mode & unix.S_IFMT
	if fileType == unix.S_IFIFO || fileType == unix.S_IFSOCK || fileType == unix.S_IFCHR || fileType == unix.S_IFBLK {
		return ErrNotRegularFile
	}

	return nil
}

func openBeneathInternal(rootDir string, reqPath string) (*os.File, string, error) {
	realRoot, rootFd, err := canonicalRoot(rootDir)
	if err != nil {
		return nil, "", err
	}
	defer unix.Close(rootFd)

	relPath, err := sanitizeRelPath(realRoot, reqPath)
	if err != nil {
		return nil, "", err
	}

	fd, err := openat2OrSecureWalk(rootFd, relPath, unix.O_RDONLY, 0)
	if err != nil {
		return nil, "", err
	}

	if err := validateDescriptor(fd); err != nil {
		_ = unix.Close(fd)
		return nil, "", err
	}

	resolved := filepath.Join(realRoot, relPath)
	return os.NewFile(uintptr(fd), resolved), resolved, nil
}

func writeBeneathInternal(rootDir string, reqPath string, data []byte, perm os.FileMode) (string, error) {
	realRoot, rootFd, err := canonicalRoot(rootDir)
	if err != nil {
		return "", err
	}
	defer unix.Close(rootFd)

	relPath, err := sanitizeRelPath(realRoot, reqPath)
	if err != nil {
		return "", err
	}

	// Garante criação segura de diretórios pais relativos ao descritor raiz
	parentDir := filepath.Dir(relPath)
	if parentDir != "." && parentDir != "" {
		if _, err := mkdirBeneathInternal(rootDir, parentDir, 0755); err != nil {
			return "", err
		}
	}

	if perm == 0 {
		perm = 0644
	}

	fd, err := openat2OrSecureWalk(rootFd, relPath, unix.O_CREAT|unix.O_TRUNC|unix.O_WRONLY, perm)
	if err != nil {
		return "", err
	}
	defer unix.Close(fd)

	if err := validateDescriptor(fd); err != nil {
		return "", err
	}

	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err == nil && (stat.Mode&unix.S_IFDIR != 0) {
		return "", ErrIsDirectory
	}

	f := os.NewFile(uintptr(fd), relPath)
	if _, err := f.Write(data); err != nil {
		return "", err
	}

	return filepath.Join(realRoot, relPath), nil
}

func createBeneathInternal(rootDir string, reqPath string, perm os.FileMode) (string, error) {
	realRoot, rootFd, err := canonicalRoot(rootDir)
	if err != nil {
		return "", err
	}
	defer unix.Close(rootFd)

	relPath, err := sanitizeRelPath(realRoot, reqPath)
	if err != nil {
		return "", err
	}

	parentDir := filepath.Dir(relPath)
	if parentDir != "." && parentDir != "" {
		if _, err := mkdirBeneathInternal(rootDir, parentDir, 0755); err != nil {
			return "", err
		}
	}

	if perm == 0 {
		perm = 0644
	}

	// O_EXCL garante que o arquivo não existe e impede seguimento de symlinks criados concorrentemente
	fd, err := openat2OrSecureWalk(rootFd, relPath, unix.O_CREAT|unix.O_EXCL|unix.O_WRONLY, perm)
	if err != nil {
		return "", err
	}
	_ = unix.Close(fd)

	return filepath.Join(realRoot, relPath), nil
}

func mkdirBeneathInternal(rootDir string, reqPath string, perm os.FileMode) (string, error) {
	realRoot, rootFd, err := canonicalRoot(rootDir)
	if err != nil {
		return "", err
	}
	defer unix.Close(rootFd)

	relPath, err := sanitizeRelPath(realRoot, reqPath)
	if err != nil {
		return "", err
	}

	if perm == 0 {
		perm = 0755
	}

	parts := strings.Split(relPath, "/")
	currPath := ""
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", ErrPathTraversal
		}
		if currPath == "" {
			currPath = part
		} else {
			currPath = filepath.Join(currPath, part)
		}

		err := unix.Mkdirat(rootFd, currPath, uint32(perm))
		if err != nil && !errors.Is(err, unix.EEXIST) {
			return "", fmt.Errorf("falha ao criar diretório via mkdirat (%s): %w", currPath, err)
		}
	}

	return filepath.Join(realRoot, relPath), nil
}

func renameBeneathInternal(rootDir string, oldPath string, newPath string) error {
	realRoot, rootFd, err := canonicalRoot(rootDir)
	if err != nil {
		return err
	}
	defer unix.Close(rootFd)

	oldRel, err := sanitizeRelPath(realRoot, oldPath)
	if err != nil {
		return fmt.Errorf("origem inválida: %w", err)
	}

	newRel, err := sanitizeRelPath(realRoot, newPath)
	if err != nil {
		return fmt.Errorf("destino inválido: %w", err)
	}

	newParent := filepath.Dir(newRel)
	if newParent != "." && newParent != "" {
		if _, err := mkdirBeneathInternal(rootDir, newParent, 0755); err != nil {
			return fmt.Errorf("diretório pai de destino inválido: %w", err)
		}
	}

	// Renameat relativo ao descritor da raiz
	return unix.Renameat(rootFd, oldRel, rootFd, newRel)
}

func removeBeneathInternal(rootDir string, reqPath string) error {
	realRoot, rootFd, err := canonicalRoot(rootDir)
	if err != nil {
		return err
	}
	defer unix.Close(rootFd)

	relPath, err := sanitizeRelPath(realRoot, reqPath)
	if err != nil {
		return err
	}

	if relPath == "." || relPath == "" {
		return errors.New("proibido excluir o diretório raiz")
	}

	resolved := filepath.Join(realRoot, relPath)
	return os.RemoveAll(resolved)
}

func listBeneathInternal(rootDir string, reqPath string) ([]FileEntry, string, error) {
	f, resolved, err := openBeneathInternal(rootDir, reqPath)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, "", err
	}
	if !fi.IsDir() {
		return nil, "", errors.New("o caminho especificado não é um diretório")
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
