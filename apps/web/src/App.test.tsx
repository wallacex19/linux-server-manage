import { describe, it, expect, vi, beforeAll } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'

beforeAll(() => {
  global.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
})

describe('App & Navigation Flow', () => {
  it('renders login screen when unauthenticated', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    // Mock /api/v1/auth/me to return 401
    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes('/auth/me')) {
        return Promise.resolve({
          ok: false,
          status: 401,
          json: () => Promise.resolve({ error: 'não autenticado' }),
        })
      }
      if (url.includes('/nodes')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve([]),
        })
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve({}) })
    })

    render(
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>,
    )

    await waitFor(() => {
      expect(screen.getByText('Linux Server Manager')).toBeDefined()
      expect(screen.getByText('Usuário Linux')).toBeDefined()
      expect(screen.getByText('Senha do Sistema')).toBeDefined()
    })
  })

  it('renders dashboard and full navigation sidebar when authenticated', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    // Mock /api/v1/auth/me to return authenticated user
    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes('/auth/me')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              username: 'admin',
              uid: 1000,
              gid: 1000,
              groups: ['admin', 'sudo'],
              home_dir: '/home/admin',
              shell: '/bin/bash',
              is_admin: true,
            }),
        })
      }
      if (url.includes('/health')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              status: 'healthy',
              timestamp: new Date().toISOString(),
              environment: 'development',
              database: 'connected',
              version: '0.1.0',
            }),
        })
      }
      if (url.includes('/system/info')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              hostname: 'ubuntu-srv-01',
              os: 'linux',
              architecture: 'amd64',
              cpu_cores: 4,
              go_version: 'go1.23.0',
              uptime_seconds: 7200,
            }),
        })
      }
      if (url.includes('/nodes')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve([]),
        })
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve({}) })
    })

    render(
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>,
    )

    await waitFor(() => {
      // Verifica presença dos menus obrigatórios da sidebar e dashboard
      expect(screen.getAllByText('Dashboard').length).toBeGreaterThan(0)
      expect(screen.getAllByText('Servidores').length).toBeGreaterThan(0)
      expect(screen.getAllByText('Monitoramento').length).toBeGreaterThan(0)
      expect(screen.getAllByText('Processos').length).toBeGreaterThan(0)
      expect(screen.getAllByText('Gerenciador de arquivos').length).toBeGreaterThan(0)
      expect(screen.getAllByText('Terminal interativo').length).toBeGreaterThan(0)
      expect(screen.getAllByText('SteamCMD').length).toBeGreaterThan(0)
      // Verifica perfil do usuário
      expect(screen.getAllByText('admin').length).toBeGreaterThan(0)
      expect(screen.getByText('Sair')).toBeDefined()
    })
  })

  it('navigates to Monitoring view and renders real-time monitoring title', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes('/auth/me')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              username: 'admin',
              uid: 1000,
              gid: 1000,
              groups: ['admin'],
              home_dir: '/home/admin',
              shell: '/bin/bash',
              is_admin: true,
            }),
        })
      }
      if (url.includes('/metrics/current')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              timestamp: new Date().toISOString(),
              cpu: { total_percent: 24.5, cores: 4, per_core_percent: [20, 25, 30, 23] },
              memory: {
                total_bytes: 16000000000,
                used_bytes: 8000000000,
                available_bytes: 8000000000,
                used_percent: 50.0,
                swap_total_bytes: 4000000000,
                swap_used_bytes: 1000000000,
                swap_used_percent: 25.0,
              },
              disk: {
                read_bytes_per_sec: 10240,
                write_bytes_per_sec: 20480,
                partitions: [
                  {
                    mount_point: '/',
                    device: '/dev/sda1',
                    fs_type: 'ext4',
                    total_bytes: 100000000000,
                    used_bytes: 40000000000,
                    free_bytes: 60000000000,
                    used_percent: 40.0,
                  },
                ],
              },
              network: [
                {
                  name: 'eth0',
                  rx_bytes_per_sec: 5000,
                  tx_bytes_per_sec: 2000,
                  rx_total_bytes: 100000,
                  tx_total_bytes: 50000,
                },
              ],
              system: {
                hostname: 'ubuntu-srv-01',
                os: 'Linux Ubuntu',
                kernel_version: '6.8.0',
                ip_address: '192.168.1.10',
                uptime_seconds: 3600,
                load_1: 0.5,
                load_5: 0.4,
                load_15: 0.3,
                process_count: 120,
                hardware_temperature_celsius: 45.2,
              },
            }),
        })
      }
      if (url.includes('/metrics/history')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve([]),
        })
      }
      if (url.includes('/nodes')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve([]),
        })
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve({}) })
    })

    const { fireEvent } = await import('@testing-library/react')

    render(
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>,
    )

    await waitFor(() => {
      expect(screen.getAllByText('Monitoramento').length).toBeGreaterThan(0)
    })

    // Clica no botão 'Monitoramento' da sidebar
    const monitoringBtn = screen.getAllByText('Monitoramento')[0]
    fireEvent.click(monitoringBtn)

    await waitFor(() => {
      expect(
        screen.getByText('Monitoramento de Recursos em Tempo Real'),
      ).toBeDefined()
      expect(screen.getByText('Taxa de amostragem:')).toBeDefined()
    })
  })

  it('navigates to Processes, Services and SteamCMD views smoothly', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes('/auth/me')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              username: 'admin',
              uid: 1000,
              gid: 1000,
              groups: ['admin'],
              home_dir: '/home/admin',
              shell: '/bin/bash',
              is_admin: true,
            }),
        })
      }
      if (url.includes('/system/processes')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve([
              {
                pid: 1,
                user: 'root',
                uid: 0,
                name: 'systemd',
                command: '/sbin/init',
                cpu_percent: 0.1,
                memory_bytes: 10485760,
                memory_percent: 0.2,
                state: 'S',
              },
            ]),
        })
      }
      if (url.includes('/system/services')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve([
              {
                name: 'sshd.service',
                description: 'OpenSSH Server Daemon',
                load_state: 'loaded',
                active_state: 'active',
                sub_state: 'running',
                can_control: true,
              },
            ]),
        })
      }
      if (url.includes('/steamcmd/status')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              installed: true,
              path: '/usr/games/steamcmd',
              version: 'Steam Console Client',
              libs_32bit_ready: true,
              os_distribution: 'linux/amd64',
              install_guidance: 'ok',
            }),
        })
      }
      if (url.includes('/steamcmd/presets')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve([
              {
                name: 'Counter-Strike 2',
                app_id: '730',
                service_type: 'cs2',
                default_port: 27015,
                default_args: './game/bin/linuxsteamrt64/cs2',
                anonymous: true,
                description: 'Servidor oficial Valve',
              },
            ]),
        })
      }
      if (url.includes('/games/servers')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve([]),
        })
      }
      if (url.includes('/nodes')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve([]),
        })
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve({}) })
    })

    const { fireEvent } = await import('@testing-library/react')

    render(
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>,
    )

    await waitFor(() => {
      expect(screen.getAllByText('Processos').length).toBeGreaterThan(0)
    })

    // 1. Navega para Processos
    fireEvent.click(screen.getAllByText('Processos')[0])
    await waitFor(() => {
      expect(screen.getByText('Processos em Execução')).toBeDefined()
    })

    // 2. Navega para Serviços
    fireEvent.click(screen.getAllByText('Serviços do Linux')[0])
    await waitFor(() => {
      expect(screen.getByText('Serviços do Linux (systemd)')).toBeDefined()
    })

    // 3. Navega para SteamCMD
    fireEvent.click(screen.getAllByText('SteamCMD')[0])
    await waitFor(() => {
      expect(screen.getByText('SteamCMD e Servidores de Jogos')).toBeDefined()
      expect(screen.getByText('Catálogo de Jogos Suportados')).toBeDefined()
    })

    // 4. Navega para Servidores
    fireEvent.click(screen.getAllByText('Servidores')[0])
    await waitFor(() => {
      expect(screen.getByText('Servidores Linux Gerenciados')).toBeDefined()
    })
  })
})
