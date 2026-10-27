package files

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linux-server-manager/backend/internal/auth"
)

func TestFileServiceLifecycleAndSecurity(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lsm_files_test_*")
	if err != nil {
		t.Fatalf("falha ao criar temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	service := NewFileService(tempDir)

	normalUser := &auth.LinuxUserIdentity{
		Username: "operador",
		UID:      1001,
		HomeDir:  tempDir,
		IsAdmin:  false,
	}

	adminUser := &auth.LinuxUserIdentity{
		Username: "admin",
		UID:      1000,
		HomeDir:  tempDir,
		IsAdmin:  true,
	}

	// 1. Testa bloqueio de Path Traversal para usuário não-admin
	_, err = service.ResolveAndValidatePath(normalUser, "../../../etc/shadow")
	if err != ErrPathTraversal {
		t.Fatalf("esperado ErrPathTraversal para operador escapando sandbox, obtido: %v", err)
	}

	// 2. Criação de subpasta e arquivo
	subDirName := "test_folder"
	if err := service.CreateDirectory(normalUser, subDirName); err != nil {
		t.Fatalf("falha ao criar pasta: %v", err)
	}

	fileName := filepath.Join(subDirName, "config.json")
	if err := service.CreateFile(normalUser, fileName); err != nil {
		t.Fatalf("falha ao criar arquivo: %v", err)
	}

	// 3. Escrita e leitura de arquivo texto
	initialContent := `{"status": "ok", "port": 8080}`
	if err := service.WriteFile(normalUser, fileName, initialContent); err != nil {
		t.Fatalf("falha ao escrever arquivo: %v", err)
	}

	contentObj, err := service.ReadFile(normalUser, fileName)
	if err != nil {
		t.Fatalf("falha ao ler arquivo: %v", err)
	}
	if contentObj.Content != initialContent {
		t.Errorf("conteúdo lido diverge do escrito: %s", contentObj.Content)
	}

	// 4. Bloqueio de arquivo binário
	binName := filepath.Join(subDirName, "binary.dat")
	binData := []byte{0x7F, 0x45, 0x4C, 0x46, 0x00, 0x02, 0x01} // byte 0x00 (ELF header)
	if err := os.WriteFile(filepath.Join(tempDir, binName), binData, 0644); err != nil {
		t.Fatalf("falha ao escrever binário de teste: %v", err)
	}

	_, err = service.ReadFile(normalUser, binName)
	if err != ErrBinaryFile {
		t.Fatalf("esperado ErrBinaryFile para arquivo com byte nulo, obtido: %v", err)
	}

	// 5. Listagem de diretório
	listing, err := service.ListDirectory(normalUser, subDirName)
	if err != nil {
		t.Fatalf("falha ao listar diretório: %v", err)
	}
	if len(listing.Entries) != 2 {
		t.Errorf("esperado 2 arquivos na pasta, obtido %d", len(listing.Entries))
	}

	// 6. Cópia e renomeação
	copiedName := filepath.Join(subDirName, "config.backup.json")
	if err := service.CopyFile(normalUser, fileName, copiedName); err != nil {
		t.Fatalf("falha ao copiar arquivo: %v", err)
	}

	renamedName := filepath.Join(subDirName, "config.renamed.json")
	if err := service.Rename(normalUser, copiedName, renamedName); err != nil {
		t.Fatalf("falha ao renomear arquivo: %v", err)
	}

	// 7. Exclusão
	if err := service.Delete(normalUser, renamedName); err != nil {
		t.Fatalf("falha ao excluir arquivo: %v", err)
	}

	// 8. Proteção de caminho crítico
	if err := service.Delete(adminUser, "/"); err != ErrCriticalTarget {
		t.Errorf("esperado ErrCriticalTarget para exclusão de raiz /, obtido: %v", err)
	}

	// 9. Prevenção de Symlink Traversal
	outsideDir, err := os.MkdirTemp("", "lsm_outside_test_*")
	if err == nil {
		defer os.RemoveAll(outsideDir)
		outsideFile := filepath.Join(outsideDir, "secret.txt")
		_ = os.WriteFile(outsideFile, []byte("super_secret_token"), 0644)

		symlinkTarget := filepath.Join(tempDir, "link_to_outside.txt")
		if symErr := os.Symlink(outsideFile, symlinkTarget); symErr == nil {
			_, valErr := service.ResolveAndValidatePath(normalUser, "link_to_outside.txt")
			if valErr != ErrPathTraversal {
				t.Errorf("esperado ErrPathTraversal para symlink apontando para fora da sandbox, obtido: %v", valErr)
			}
		}
	}
}
