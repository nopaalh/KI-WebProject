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

export type PatientProfileData = { id: number; user_id: number; weight_kg: number; height_cm: number; age: number; nik: string; bpjs_number: string }
export type CreatePatientProfilePayload = { weight_kg: number; height_cm: number; age: number; nik: string; bpjs_number: string }
export type PatientKTPData = { id: number; patient_profile_id: number; nik: string; full_name: string; birth_place: string; birth_date: string; gender: string; blood_type: string; address: string; rt: string; rw: string; village: string; district: string; religion: string; marital_status: string; occupation: string; citizenship: string }
export type CreatePatientKTPPayload = { full_name: string; birth_place: string; birth_date: string; gender: 'Laki-laki' | 'Perempuan'; blood_type: 'A' | 'B' | 'AB' | 'O' | '-'; address: string; rt: string; rw: string; village: string; district: string; religion: string; marital_status: 'Belum Kawin' | 'Kawin' | 'Cerai Hidup' | 'Cerai Mati'; occupation: string; citizenship: 'WNI' | 'WNA' }

export async function apiLogout(): Promise<void> {
  const res = await fetchWithHandling('/api/users/logout', { method: 'POST', signal: AbortSignal.timeout(15000), headers: { Accept: 'application/json' } })
  await res.json().catch(() => null)
}

export async function apiGetPatientProfile(signal?: AbortSignal): Promise<PatientProfileData> {
  const res = await fetchWithHandling('/api/patients/me/profile', { method: 'GET', signal: signal ?? AbortSignal.timeout(15000), headers: { Accept: 'application/json' } })
  const data = await res.json()
  if (!data?.profile || typeof data.profile.id !== 'number') throw new ApiError(500, 'Respons profil pasien tidak sesuai.')
  return data.profile as PatientProfileData
}

export async function apiCreatePatientProfile(payload: CreatePatientProfilePayload): Promise<void> {
  const res = await fetchWithHandling('/api/patients/me/profile', { method: 'POST', signal: AbortSignal.timeout(15000), headers: { 'Content-Type': 'application/json', Accept: 'application/json' }, body: JSON.stringify(payload) })
  await res.json().catch(() => null)
}

export async function apiGetPatientKTP(signal?: AbortSignal): Promise<PatientKTPData> {
  const res = await fetchWithHandling('/api/patients/me/ktp', { method: 'GET', signal: signal ?? AbortSignal.timeout(15000), headers: { Accept: 'application/json' } })
  const data = await res.json()
  if (!data?.ktp || typeof data.ktp.id !== 'number') throw new ApiError(500, 'Respons data KTP tidak sesuai.')
  return data.ktp as PatientKTPData
}

export async function apiGetPatientKTPPhoto(signal?: AbortSignal): Promise<Blob> {
  const res = await fetchWithHandling('/api/patients/me/ktp/photo', { method: 'GET', signal: signal ?? AbortSignal.timeout(20000) })
  return res.blob()
}

export async function apiCreatePatientKTP(data: CreatePatientKTPPayload, photo: File): Promise<void> {
  const form = new FormData()
  form.append('data', JSON.stringify(data))
  form.append('photo', photo)
  const res = await fetchWithHandling('/api/patients/me/ktp', { method: 'POST', signal: AbortSignal.timeout(30000), headers: { Accept: 'application/json' }, body: form })
  await res.json().catch(() => null)
}
