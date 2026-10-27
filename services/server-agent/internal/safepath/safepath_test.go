package safepath

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSafePath_BasicOperations(t *testing.T) {
	root := t.TempDir()

	// 1. Criar pasta
	subDir, err := MkdirBeneath(root, "docs", 0755)
	if err != nil {
		t.Fatalf("falha ao criar subdiretório: %v", err)
	}

	// 2. Criar arquivo exclusivo
	filePath, err := CreateBeneath(root, "docs/nota.txt", 0644)
	if err != nil {
		t.Fatalf("falha ao criar arquivo: %v", err)
	}

	// Tentar recriar com CreateBeneath deve falhar (O_EXCL)
	_, err = CreateBeneath(root, "docs/nota.txt", 0644)
	if err == nil {
		t.Errorf("esperado erro ao tentar recriar arquivo existente com CreateBeneath")
	}

	// 3. Gravar dados
	content := []byte("Dados confidenciais de teste")
	_, err = WriteBeneath(root, "docs/nota.txt", content, 0644)
	if err != nil {
		t.Fatalf("falha ao gravar dados: %v", err)
	}

	// 4. Ler dados
	readBuf, resolved, err := ReadBeneath(root, "docs/nota.txt", 1024)
	if err != nil {
		t.Fatalf("falha ao ler dados: %v", err)
	}
	if !bytes.Equal(readBuf, content) {
		t.Errorf("conteúdo lido diverge do gravado: %s", string(readBuf))
	}
	if resolved != filePath {
		t.Errorf("caminho resolvido diverge: esperado %s, obtido %s", filePath, resolved)
	}

	// 5. Listar diretório
	entries, _, err := ListBeneath(root, "docs")
	if err != nil {
		t.Fatalf("falha ao listar diretório: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "nota.txt" {
		t.Errorf("listagem inesperada: %+v", entries)
	}

	// 6. Renomear arquivo
	err = RenameBeneath(root, "docs/nota.txt", "docs/nota_renomeada.txt")
	if err != nil {
		t.Fatalf("falha ao renomear arquivo: %v", err)
	}

	// 7. Remover arquivo e pasta
	err = RemoveBeneath(root, "docs/nota_renomeada.txt")
	if err != nil {
		t.Fatalf("falha ao excluir arquivo: %v", err)
	}

	err = RemoveBeneath(root, subDir)
	if err != nil {
		t.Fatalf("falha ao excluir pasta: %v", err)
	}
}

func TestSafePath_PathTraversal_Attempts(t *testing.T) {
	root := t.TempDir()

	traversalAttempts := []string{
		"../outside.txt",
		"../../etc/passwd",
		"../../../etc/shadow",
		"docs/../../outside.txt",
		"nested/sub/../../../etc/hosts",
	}

	for _, att := range traversalAttempts {
		_, _, err := ReadBeneath(root, att, 1024)
		if err == nil {
			t.Errorf("esperado erro de traversal para %s, mas sucedeu", att)
		}
		if !errors.Is(err, ErrPathTraversal) && !errors.Is(err, ErrSymlinkEscape) {
			t.Errorf("erro deve ser ErrPathTraversal ou ErrSymlinkEscape para %s, obtido: %v", att, err)
		}

		_, err = WriteBeneath(root, att, []byte("ataque"), 0644)
		if err == nil {
			t.Errorf("esperado bloqueio na escrita para %s, mas sucedeu", att)
		}

		_, err = CreateBeneath(root, att, 0644)
		if err == nil {
			t.Errorf("esperado bloqueio na criação para %s, mas sucedeu", att)
		}

		err = RemoveBeneath(root, att)
		if err == nil {
			t.Errorf("esperado bloqueio na exclusão para %s, mas sucedeu", att)
		}
	}
}

func TestSafePath_AbsolutePathsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	secretFile := filepath.Join(outsideDir, "secret.txt")
	_ = os.WriteFile(secretFile, []byte("super-secret"), 0600)

	// Tentar acessar caminho absoluto que não está sob root
	_, _, err := ReadBeneath(root, secretFile, 1024)
	if err == nil {
		t.Fatalf("esperado bloqueio para caminho absoluto fora da raiz: %s", secretFile)
	}

	_, err = WriteBeneath(root, secretFile, []byte("overwrite"), 0600)
	if err == nil {
		t.Fatalf("esperado bloqueio para gravação fora da raiz: %s", secretFile)
	}
}

func TestSafePath_SymlinkEscape_Prevention(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	outsideSecret := filepath.Join(outsideDir, "secret.txt")
	_ = os.WriteFile(outsideSecret, []byte("conteudo confidencial do host"), 0600)

	// Cria um link simbólico dentro de root apontando para o arquivo de fora
	symlinkTarget := filepath.Join(root, "link_fora.txt")
	err := os.Symlink(outsideSecret, symlinkTarget)
	if err != nil {
		t.Skipf("symlink não suportado no ambiente: %v", err)
	}

	// 1. Tentar ler através do symlink
	_, _, err = ReadBeneath(root, "link_fora.txt", 1024)
	if err == nil {
		t.Fatalf("FALHA DE SEGURANÇA: symlink para fora da raiz foi lido com sucesso")
	}
	if !errors.Is(err, ErrSymlinkEscape) && !errors.Is(err, ErrPathTraversal) {
		t.Logf("Aviso: erro recebido no escape de symlink: %v", err)
	}

	// 2. Tentar escrever através do symlink
	_, err = WriteBeneath(root, "link_fora.txt", []byte("ataque"), 0644)
	if err == nil {
		t.Fatalf("FALHA DE SEGURANÇA: symlink para fora da raiz permitiu gravação")
	}

	// 3. Symlink intermediário de diretório
	dirSymlink := filepath.Join(root, "pasta_link")
	err = os.Symlink(outsideDir, dirSymlink)
	if err == nil {
		_, _, err = ReadBeneath(root, "pasta_link/secret.txt", 1024)
		if err == nil {
			t.Fatalf("FALHA DE SEGURANÇA: diretório intermediário com symlink permitiu leitura de fora")
		}
	}
}

func TestSafePath_ConcurrentTOCTOU_Race(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	sensitiveFile := filepath.Join(outsideDir, "vault.txt")
	sensitiveContent := []byte("SENSITIVE_VAULT_DATA")
	_ = os.WriteFile(sensitiveFile, sensitiveContent, 0600)

	sharedSubdir := filepath.Join(root, "shared")
	_ = os.MkdirAll(sharedSubdir, 0755)
	_ = os.WriteFile(filepath.Join(sharedSubdir, "safe.txt"), []byte("SAFE_CONTENT"), 0644)

	var stopFlag int32
	var escapeDetected int32

	// Goroutine que tenta alternar concorrentemente o diretório compartilhado com um link simbólico para outsideDir
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for atomic.LoadInt32(&stopFlag) == 0 {
			// Substitui pasta real por symlink para pasta de fora
			_ = os.RemoveAll(sharedSubdir)
			_ = os.Symlink(outsideDir, sharedSubdir)
			time.Sleep(50 * time.Microsecond)

			// Restaura pasta real
			_ = os.Remove(sharedSubdir)
			_ = os.MkdirAll(sharedSubdir, 0755)
			_ = os.WriteFile(filepath.Join(sharedSubdir, "safe.txt"), []byte("SAFE_CONTENT"), 0644)
			time.Sleep(50 * time.Microsecond)
		}
	}()

	// 10 Goroutines de leitura/escrita simultâneas
	numReaders := 10
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				// Tenta ler o arquivo através do caminho que está sofrendo a corrida
				data, _, err := ReadBeneath(root, "shared/vault.txt", 1024)
				if err == nil {
					// Se ler com sucesso, NUNCA pode ter o conteúdo confidencial do cofre!
					if bytes.Equal(data, sensitiveContent) {
						atomic.StoreInt32(&escapeDetected, 1)
					}
				}

				// Tenta escrever
				_, err = WriteBeneath(root, "shared/attack.txt", []byte("INTRUSION"), 0644)
				if err == nil {
					// Verifica se o arquivo foi parar fora da raiz
					if _, statErr := os.Stat(filepath.Join(outsideDir, "attack.txt")); statErr == nil {
						atomic.StoreInt32(&escapeDetected, 1)
						_ = os.Remove(filepath.Join(outsideDir, "attack.txt"))
					}
				}
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)
	atomic.StoreInt32(&stopFlag, 1)
	wg.Wait()

	if atomic.LoadInt32(&escapeDetected) != 0 {
		t.Fatalf("FALHA CRÍTICA DE TOCTOU: Operação conseguiu escapar da raiz autorizada durante corrida concorrente de symlink!")
	}
}
