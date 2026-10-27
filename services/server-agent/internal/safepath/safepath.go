package safepath

import (
	"errors"
	"io"
	"os"
	"time"
)

var (
	ErrPathTraversal       = errors.New("acesso negado: tentativa de path traversal fora da raiz autorizada")
	ErrSymlinkEscape       = errors.New("acesso negado: link simbólico aponta para fora da raiz autorizada")
	ErrCriticalPathBlocked = errors.New("acesso negado: operação bloqueada em caminho crítico do sistema")
	ErrFileTooLarge        = errors.New("arquivo excede o limite máximo permitido de 5 MB")
	ErrIsDirectory         = errors.New("o caminho especificado é um diretório")
	ErrNotRegularFile      = errors.New("o caminho especificado não é um arquivo regular (FIFO, socket ou dispositivo bloqueado)")
	ErrUnsupportedKernel   = errors.New("sistema operacional ou kernel não suporta controle de segurança openat2/RESOLVE_BENEATH")
)

const MaxFileSize = 5 * 1024 * 1024 // 5 MB

// FileEntry representa uma entrada de diretório validada
type FileEntry struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	IsDir      bool      `json:"is_dir"`
	Size       int64     `json:"size"`
	Mode       string    `json:"mode"`
	ModifiedAt time.Time `json:"modified_at"`
}

// OpenBeneath abre um arquivo existente sob rootDir garantindo restrição pelo kernel (openat2 no Linux)
func OpenBeneath(rootDir string, reqPath string) (*os.File, string, error) {
	return openBeneathInternal(rootDir, reqPath)
}

// ReadBeneath lê até maxBytes de um arquivo garantindo que não escape de rootDir
func ReadBeneath(rootDir string, reqPath string, maxBytes int64) ([]byte, string, error) {
	if maxBytes <= 0 || maxBytes > MaxFileSize {
		maxBytes = MaxFileSize
	}

	f, resolved, err := OpenBeneath(rootDir, reqPath)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, "", err
	}
	if fi.IsDir() {
		return nil, "", ErrIsDirectory
	}
	if fi.Size() > maxBytes {
		return nil, "", ErrFileTooLarge
	}

	buf := make([]byte, fi.Size())
	_, err = io.ReadFull(f, buf)
	if err != nil && err != io.EOF {
		return nil, "", err
	}

	return buf, resolved, nil
}

// WriteBeneath grava dados atomicamente em um arquivo sob rootDir
func WriteBeneath(rootDir string, reqPath string, data []byte, perm os.FileMode) (string, error) {
	return writeBeneathInternal(rootDir, reqPath, data, perm)
}

// CreateBeneath cria exclusivamente um novo arquivo (O_CREAT|O_EXCL) sob rootDir
func CreateBeneath(rootDir string, reqPath string, perm os.FileMode) (string, error) {
	return createBeneathInternal(rootDir, reqPath, perm)
}

// MkdirBeneath cria um diretório sob rootDir
func MkdirBeneath(rootDir string, reqPath string, perm os.FileMode) (string, error) {
	return mkdirBeneathInternal(rootDir, reqPath, perm)
}

// RenameBeneath renomeia arquivos/diretórios garantindo que ambos permaneçam sob rootDir
func RenameBeneath(rootDir string, oldPath string, newPath string) error {
	return renameBeneathInternal(rootDir, oldPath, newPath)
}

// RemoveBeneath exclui arquivos ou diretórios sob rootDir
func RemoveBeneath(rootDir string, reqPath string) error {
	return removeBeneathInternal(rootDir, reqPath)
}

// ListBeneath lista as entradas de um diretório sob rootDir
func ListBeneath(rootDir string, reqPath string) ([]FileEntry, string, error) {
	return listBeneathInternal(rootDir, reqPath)
}
