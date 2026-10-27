import type { DocumentCategory } from './patientExamples'

export type UserRole = 'patient' | 'doctor'
export type CurrentUser = { id: number; name: string; username: string; role: UserRole }
export type CipherAlgorithm = 'aes' | 'des' | 'rc4'

export class ApiError extends Error {
  status: number
  isNetworkError: boolean
  constructor(status: number, message: string, isNetworkError = false) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.isNetworkError = isNetworkError
  }
}

export function classifyFileKind(name: string): string {
  const ext = name.split('.').pop()?.trim().toUpperCase() ?? ''
  return ext ? ext.slice(0, 8) : 'FILE'
}

export function classifyFileCategory(name: string, mimeType: string): DocumentCategory {
  const lowerMime = (mimeType || '').toLowerCase()
  const lowerName = name.toLowerCase()
  if (lowerMime.startsWith('image/') || /\.(png|jpe?g|webp|gif)$/i.test(lowerName)) return 'Foto'
  if (lowerMime.startsWith('video/') || /\.(mp4|webm|mov)$/i.test(lowerName)) return 'Video'
  return 'Dokumen'
}

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1048576) return `${(bytes / 1024).toLocaleString('id-ID', { maximumFractionDigits: 0 })} KB`
  return `${(bytes / 1048576).toLocaleString('id-ID', { maximumFractionDigits: 1 })} MB`
}

export function formatNsToMs(ns: number | null | undefined): string {
  if (typeof ns !== 'number' || !Number.isFinite(ns) || ns <= 0) return 'Belum diukur'
  return `${(ns / 1e6).toLocaleString('id-ID', { minimumFractionDigits: 2, maximumFractionDigits: 3 })} ms`
}

export function sanitizeDownloadFilename(name: string): string {
  return name.replace(/[\u0000-\u001f\u007f/\\]/g, '_').trim() || 'lampiran.bin'
}

export async function fetchWithHandling(input: string, init: RequestInit = {}): Promise<Response> {
  let response: Response
  try {
    response = await fetch(input, { ...init, credentials: 'include', cache: 'no-store' })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError(0, 'Tidak dapat terhubung ke layanan.', true)
  }
  if (!response.ok) {
    const data = await response.json().catch(() => null)
    const msg = typeof data?.message === 'string' && data.message.trim() ? data.message.trim() : `Permintaan gagal (HTTP ${response.status})`
    throw new ApiError(response.status, msg, false)
  }
  return response
}
