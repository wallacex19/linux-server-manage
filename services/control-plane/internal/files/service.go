package files

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/linux-server-manager/backend/internal/auth"
)

var (
	ErrAccessDenied    = errors.New("acesso negado: permissões insuficientes ou fora do escopo autorizado")
	ErrPathTraversal   = errors.New("tentativa de path traversal detectada")
	ErrBinaryFile      = errors.New("o arquivo selecionado é binário e não pode ser editado como texto")
	ErrCriticalTarget  = errors.New("operação bloqueada em diretório ou arquivo crítico do sistema")
	ErrTargetExists    = errors.New("o destino já existe")
	ErrFileNotFound    = errors.New("arquivo ou diretório não encontrado")
)

type FileService struct {
	baseDir string // Opcional, para testes ou jaula de sandbox
}

func NewFileService(baseDir string) *FileService {
	return &FileService{baseDir: baseDir}
}

// ResolveAndValidatePath resolve o caminho canônico e garante que não haja escape do escopo permitido
func (s *FileService) ResolveAndValidatePath(user *auth.LinuxUserIdentity, reqPath string) (string, error) {
	if user == nil {
		return "", ErrAccessDenied
	}

	cleanReq := filepath.Clean(reqPath)
	if cleanReq == "." || cleanReq == "" {
		if s.baseDir != "" {
			cleanReq = s.baseDir
		} else if user.HomeDir != "" {
			cleanReq = user.HomeDir
		} else {
			cleanReq = "/"
		}
	}

	// Converte para absoluto
	var targetPath string
	if cleanReq == "/" || cleanReq == "\\" {
		targetPath = "/"
	} else if filepath.IsAbs(cleanReq) {
		targetPath = cleanReq
	} else {
		base := user.HomeDir
		if s.baseDir != "" {
			base = s.baseDir
		}
		targetPath = filepath.Join(base, cleanReq)
	}

	targetPath = filepath.Clean(targetPath)

	// Se não for admin, restringe estritamente ao diretório home do usuário (ou baseDir)
	if !user.IsAdmin {
		allowedRoot := user.HomeDir
		if s.baseDir != "" {
			allowedRoot = s.baseDir
		}
		allowedRoot = filepath.Clean(allowedRoot)

		rel, err := filepath.Rel(allowedRoot, targetPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", ErrPathTraversal
		}

		// Prevenção contra Symlink Traversal:
		// Avalia o caminho real resolvendo links simbólicos da raiz permitida
		realRoot, err := filepath.EvalSymlinks(allowedRoot)
		if err != nil {
			realRoot = allowedRoot
		}

		// Se o caminho de destino existe no sistema de arquivos, avalia seus links simbólicos
		if realTarget, err := filepath.EvalSymlinks(targetPath); err == nil {
			relReal, errRel := filepath.Rel(realRoot, realTarget)
			if errRel != nil || strings.HasPrefix(relReal, "..") {
				return "", ErrPathTraversal
			}
			return realTarget, nil
		} else {
			// Se o destino ainda não existe (ex: criação de novo arquivo/pasta),
			// resolve links simbólicos do diretório pai existente mais próximo
			dir := filepath.Dir(targetPath)
			subPath := filepath.Base(targetPath)
			for {
				if realDir, err := filepath.EvalSymlinks(dir); err == nil {
					relDir, errRel := filepath.Rel(realRoot, realDir)
					if errRel != nil || strings.HasPrefix(relDir, "..") {
						return "", ErrPathTraversal
					}
					return filepath.Join(realDir, subPath), nil
				}
				parent := filepath.Dir(dir)
				if parent == dir {
					break
				}
				subPath = filepath.Join(filepath.Base(dir), subPath)
				dir = parent
			}
		}
	}

	return targetPath, nil
}

// ListDirectory lista os arquivos e subdiretórios com atributos POSIX
func (s *FileService) ListDirectory(user *auth.LinuxUserIdentity, reqPath string) (*DirectoryListing, error) {
	targetPath, err := s.ResolveAndValidatePath(user, reqPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao inspecionar diretório: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("o caminho especificado não é um diretório")
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return nil, fmt.Errorf("erro de permissão ao ler diretório: %w", err)
	}

	var fileList []FileEntry
	for _, entry := range entries {
		itemInfo, err := entry.Info()
		if err != nil {
			continue
		}

		fullItemPath := filepath.Join(targetPath, entry.Name())
		isSymlink := (itemInfo.Mode() & os.ModeSymlink) != 0

		fileList = append(fileList, FileEntry{
			Name:        entry.Name(),
			Path:        fullItemPath,
			IsDir:       entry.IsDir(),
			IsSymlink:   isSymlink,
			Size:        itemInfo.Size(),
			Permissions: itemInfo.Mode().String(),
			ModTime:     itemInfo.ModTime().UTC(),
			MimeType:    detectMime(entry.Name(), entry.IsDir()),
			IsBinary:    isBinaryName(entry.Name()),
		})
	}

	parentPath := filepath.Dir(targetPath)
	if parentPath == targetPath {
		parentPath = ""
	}

	return &DirectoryListing{
		CurrentPath: targetPath,
		ParentPath:  parentPath,
		Entries:     fileList,
		TotalItems:  len(fileList),
	}, nil
}

// ReadFile lê um arquivo como texto UTF-8, impedindo abertura de binários
func (s *FileService) ReadFile(user *auth.LinuxUserIdentity, reqPath string) (*FileContent, error) {
	targetPath, err := s.ResolveAndValidatePath(user, reqPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, ErrFileNotFound
	}
	if info.IsDir() {
		return nil, fmt.Errorf("caminho aponta para um diretório, não um arquivo")
	}

	// Limite de 5 MB para edição direta no navegador
	if info.Size() > 5*1024*1024 {
		return nil, fmt.Errorf("arquivo muito grande para o editor web (máximo: 5 MB)")
	}

	// #nosec G304 -- targetPath validado contra a raiz autorizada do usuário por ResolveAndValidatePath
	rawBytes, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler arquivo: %w", err)
	}

	if isBinaryContent(rawBytes) {
		return nil, ErrBinaryFile
	}

	return &FileContent{
		Path:        targetPath,
		Content:     string(rawBytes),
		Size:        info.Size(),
		Permissions: info.Mode().String(),
		ModTime:     info.ModTime().UTC(),
		IsBinary:    false,
	}, nil
}

// WriteFile salva conteúdo em arquivo texto respeitando permissões
func (s *FileService) WriteFile(user *auth.LinuxUserIdentity, reqPath, content string) error {
	targetPath, err := s.ResolveAndValidatePath(user, reqPath)
	if err != nil {
		return err
	}

	if isCriticalSystemPath(targetPath) {
		return ErrCriticalTarget
	}

	// Preserva permissões existentes se o arquivo já existir
	var perm os.FileMode = 0644
	if info, err := os.Stat(targetPath); err == nil {
		perm = info.Mode().Perm()
	}

	return os.WriteFile(targetPath, []byte(content), perm)
}

// CreateDirectory cria nova pasta de forma segura
func (s *FileService) CreateDirectory(user *auth.LinuxUserIdentity, reqPath string) error {
	targetPath, err := s.ResolveAndValidatePath(user, reqPath)
	if err != nil {
		return err
	}
	return os.MkdirAll(targetPath, 0750)
}

// CreateFile cria um arquivo de texto vazio
func (s *FileService) CreateFile(user *auth.LinuxUserIdentity, reqPath string) error {
	targetPath, err := s.ResolveAndValidatePath(user, reqPath)
	if err != nil {
		return err
	}

	if _, err := os.Stat(targetPath); err == nil {
		return ErrTargetExists
	}

	// #nosec G304 -- targetPath validado por ResolveAndValidatePath
	f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	return f.Close()
}

// Rename renomeia ou move arquivos/diretórios
func (s *FileService) Rename(user *auth.LinuxUserIdentity, oldPath, newPath string) error {
	src, err := s.ResolveAndValidatePath(user, oldPath)
	if err != nil {
		return err
	}
	dst, err := s.ResolveAndValidatePath(user, newPath)
	if err != nil {
		return err
	}

	if isCriticalSystemPath(src) || isCriticalSystemPath(dst) {
		return ErrCriticalTarget
	}

	return os.Rename(src, dst)
}

// Delete exclui arquivos ou pastas
func (s *FileService) Delete(user *auth.LinuxUserIdentity, reqPath string) error {
	targetPath, err := s.ResolveAndValidatePath(user, reqPath)
	if err != nil {
		return err
	}

	if isCriticalSystemPath(targetPath) {
		return ErrCriticalTarget
	}

	return os.RemoveAll(targetPath)
}

// CopyFile copia um arquivo de src para dst
func (s *FileService) CopyFile(user *auth.LinuxUserIdentity, srcPath, dstPath string) error {
	src, err := s.ResolveAndValidatePath(user, srcPath)
	if err != nil {
		return err
	}
	dst, err := s.ResolveAndValidatePath(user, dstPath)
	if err != nil {
		return err
	}

	// #nosec G304 -- src validado por ResolveAndValidatePath
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("cópia recursiva de diretórios não suportada nesta ação")
	}

	// #nosec G304 -- dst validado por ResolveAndValidatePath
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func isBinaryContent(data []byte) bool {
	// Limita aos primeiros 512 bytes para detecção rápida
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	// Se tiver byte nulo 0x00, é binário
	if bytes.IndexByte(sample, 0) != -1 {
		return true
	}
	// Confere MIME
	mime := http.DetectContentType(sample)
	if strings.HasPrefix(mime, "text/") || strings.Contains(mime, "json") || strings.Contains(mime, "xml") || strings.Contains(mime, "javascript") {
		return false
	}
	// Tipos comuns aceitos como texto
	return mime == "application/octet-stream"
}

func isBinaryName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	binExts := map[string]bool{
		".exe": true, ".bin": true, ".so": true, ".dll": true, ".iso": true,
		".zip": true, ".tar": true, ".gz": true, ".7z": true, ".png": true,
		".jpg": true, ".jpeg": true, ".gif": true, ".mp4": true, ".mp3": true,
	}
	return binExts[ext]
}

func detectMime(name string, isDir bool) string {
	if isDir {
		return "directory"
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "text/yaml"
	case ".sh", ".bash":
		return "text/x-shellscript"
	case ".go":
		return "text/x-go"
	case ".ts", ".tsx":
		return "text/typescript"
	case ".js", ".jsx":
		return "text/javascript"
	case ".txt", ".log", ".cfg", ".ini", ".conf", ".service":
		return "text/plain"
	case ".md":
		return "text/markdown"
	default:
		return "application/octet-stream"
	}
}

func isCriticalSystemPath(p string) bool {
	cleaned := filepath.Clean(p)
	if cleaned == "/" || cleaned == "\\" || cleaned == "." || (len(cleaned) <= 3 && strings.HasSuffix(cleaned, ":\\")) {
		return true
	}
	critical := []string{
		"/", "/etc", "/bin", "/sbin", "/usr", "/boot", "/dev", "/lib", "/lib64",
		"/proc", "/sys", "/var", "/root",
		"C:\\", "C:\\Windows", "C:\\Windows\\System32",
	}
	for _, c := range critical {
		if strings.EqualFold(cleaned, filepath.Clean(c)) {
			return true
		}
	}
	return false
}
