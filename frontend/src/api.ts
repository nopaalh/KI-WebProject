import type { DocumentCategory } from './patientExamples'

export type UserRole = 'patient' | 'doctor'

export type CurrentUser = {
  id: number
  name: string
  username: string
  role: UserRole
}

export type CipherAlgorithm = 'aes' | 'des' | 'rc4'

export type ApiFileRecord = {
  id: number
  uploaded_by_user_id: number
  patient_profile_id: number | null
  original_name: string
  original_size: number
  original_type: string
  algorithm: string
  algorithms: CipherAlgorithm[]
  cipher_set_version: number
  aes_encrypt_time_ns: number
  aes_decrypt_time_ns: number
  des_encrypt_time_ns: number
  des_decrypt_time_ns: number
  rc4_encrypt_time_ns: number
  rc4_decrypt_time_ns: number
}

export type PatientProfileData = {
  id: number
  user_id: number
  weight_kg: number
  height_cm: number
  age: number
  nik: string
  bpjs_number: string
}

export type CreatePatientProfilePayload = {
  weight_kg: number
  height_cm: number
  age: number
  nik: string
  bpjs_number: string
}

export type PatientKTPData = {
  id: number
  patient_profile_id: number
  nik: string
  full_name: string
  birth_place: string
  birth_date: string
  gender: string
  blood_type: string
  address: string
  rt: string
  rw: string
  village: string
  district: string
  religion: string
  marital_status: string
  occupation: string
  citizenship: string
}

export type CreatePatientKTPPayload = {
  full_name: string
  birth_place: string
  birth_date: string
  gender: 'Laki-laki' | 'Perempuan'
  blood_type: 'A' | 'B' | 'AB' | 'O' | '-'
  address: string
  rt: string
  rw: string
  village: string
  district: string
  religion: string
  marital_status: 'Belum Kawin' | 'Kawin' | 'Cerai Hidup' | 'Cerai Mati'
  occupation: string
  citizenship: 'WNI' | 'WNA'
}

export type DoctorPatientSummary = {
  patient_profile_id: number
  user_id: number
  name: string
  username: string
}

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
  if (lowerMime.startsWith('image/') || /\.(png|jpe?g|webp|gif)$/i.test(lowerName)) {
    return 'Foto'
  }
  if (lowerMime.startsWith('video/') || /\.(mp4|webm|mov)$/i.test(lowerName)) {
    return 'Video'
  }
  return 'Dokumen'
}

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1048576) {
    return `${(bytes / 1024).toLocaleString('id-ID', { maximumFractionDigits: 0 })} KB`
  }
  return `${(bytes / 1048576).toLocaleString('id-ID', { maximumFractionDigits: 1 })} MB`
}

export function formatNsToMs(ns: number | null | undefined): string {
  if (typeof ns !== 'number' || !Number.isFinite(ns) || ns <= 0) {
    return 'Belum diukur'
  }
  const ms = ns / 1e6
  return `${ms.toLocaleString('id-ID', { minimumFractionDigits: 2, maximumFractionDigits: 3 })} ms`
}

export function sanitizeDownloadFilename(name: string): string {
  const cleaned = name
    .replace(/[\u0000-\u001f\u007f/\\]/g, '_')
    .trim()
  return cleaned || 'lampiran.bin'
}

async function parseErrorMessage(response: Response): Promise<string> {
  const data = await response.json().catch(() => null)
  if (data && typeof data.message === 'string' && data.message.trim()) {
    return data.message.trim()
  }
  return `Permintaan gagal (HTTP ${response.status})`
}

async function fetchWithHandling(input: string, init: RequestInit = {}): Promise<Response> {
  let response: Response
  try {
    response = await fetch(input, {
      ...init,
      credentials: 'include',
      cache: 'no-store',
    })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw err
    }
    throw new ApiError(0, 'Tidak dapat terhubung ke layanan. Periksa jaringan atau server backend.', true)
  }

  if (!response.ok) {
    const message = await parseErrorMessage(response)
    throw new ApiError(response.status, message, false)
  }

  return response
}

export async function apiLogout(): Promise<void> {
  const response = await fetchWithHandling('/api/users/logout', {
    method: 'POST',
    signal: AbortSignal.timeout(15000),
    headers: { Accept: 'application/json' },
  })
  await response.json().catch(() => null)
}

export async function apiListFiles(signal?: AbortSignal): Promise<ApiFileRecord[]> {
  const response = await fetchWithHandling('/api/files', {
    method: 'GET',
    signal: signal ?? AbortSignal.timeout(15000),
    headers: { Accept: 'application/json' },
  })
  const data = await response.json()
  if (!data || !Array.isArray(data.files)) {
    throw new ApiError(500, 'Respons daftar berkas tidak sesuai.')
  }
  return data.files as ApiFileRecord[]
}

export async function apiGetFileMetadata(fileId: number, signal?: AbortSignal): Promise<ApiFileRecord> {
  const response = await fetchWithHandling(`/api/files/${encodeURIComponent(fileId)}`, {
    method: 'GET',
    signal: signal ?? AbortSignal.timeout(15000),
    headers: { Accept: 'application/json' },
  })
  const data = await response.json()
  if (!data || !data.file || typeof data.file.id !== 'number') {
    throw new ApiError(500, 'Respons metadata berkas tidak sesuai.')
  }
  return data.file as ApiFileRecord
}

export async function apiUploadPatientFile(file: File): Promise<ApiFileRecord> {
  const form = new FormData()
  form.append('file', file)
  const response = await fetchWithHandling('/api/files', {
    method: 'POST',
    signal: AbortSignal.timeout(30000),
    headers: { Accept: 'application/json' },
    body: form,
  })
  const data = await response.json()
  if (!data || !data.file || typeof data.file.id !== 'number') {
    throw new ApiError(500, 'Respons unggah berkas tidak sesuai.')
  }
  return data.file as ApiFileRecord
}

export async function apiDecryptFile(
  fileId: number,
  algorithm: CipherAlgorithm = 'aes',
  signal?: AbortSignal,
): Promise<{ blob: Blob; decryptTimeNs: number | null; algorithm: CipherAlgorithm }> {
  const query = new URLSearchParams({ algorithm })
  const response = await fetchWithHandling(`/api/files/${encodeURIComponent(fileId)}/decrypt?${query.toString()}`, {
    method: 'POST',
    signal: signal ?? AbortSignal.timeout(30000),
  })
  const timingHeader = response.headers.get('X-Decryption-Time-Ns')
  const parsedTiming = timingHeader !== null && timingHeader !== '' ? Number(timingHeader) : null
  const decryptTimeNs = parsedTiming !== null && Number.isFinite(parsedTiming) ? parsedTiming : null
  const algoHeader = response.headers.get('X-Decryption-Algorithm')
  const resolvedAlgorithm: CipherAlgorithm =
    algoHeader === 'des' || algoHeader === 'rc4' || algoHeader === 'aes' ? algoHeader : algorithm
  const blob = await response.blob()
  return { blob, decryptTimeNs, algorithm: resolvedAlgorithm }
}

export async function apiGetPatientProfile(signal?: AbortSignal): Promise<PatientProfileData> {
  const response = await fetchWithHandling('/api/patients/me/profile', {
    method: 'GET',
    signal: signal ?? AbortSignal.timeout(15000),
    headers: { Accept: 'application/json' },
  })
  const data = await response.json()
  if (!data || !data.profile || typeof data.profile.id !== 'number') {
    throw new ApiError(500, 'Respons profil pasien tidak sesuai.')
  }
  return data.profile as PatientProfileData
}

export async function apiCreatePatientProfile(payload: CreatePatientProfilePayload): Promise<void> {
  const response = await fetchWithHandling('/api/patients/me/profile', {
    method: 'POST',
    signal: AbortSignal.timeout(15000),
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: JSON.stringify(payload),
  })
  await response.json().catch(() => null)
}

export async function apiGetPatientKTP(signal?: AbortSignal): Promise<PatientKTPData> {
  const response = await fetchWithHandling('/api/patients/me/ktp', {
    method: 'GET',
    signal: signal ?? AbortSignal.timeout(15000),
    headers: { Accept: 'application/json' },
  })
  const data = await response.json()
  if (!data || !data.ktp || typeof data.ktp.id !== 'number') {
    throw new ApiError(500, 'Respons data KTP tidak sesuai.')
  }
  return data.ktp as PatientKTPData
}

export async function apiGetPatientKTPPhoto(signal?: AbortSignal): Promise<Blob> {
  const response = await fetchWithHandling('/api/patients/me/ktp/photo', {
    method: 'GET',
    signal: signal ?? AbortSignal.timeout(20000),
  })
  return response.blob()
}

export async function apiCreatePatientKTP(data: CreatePatientKTPPayload, photo: File): Promise<void> {
  const form = new FormData()
  form.append('data', JSON.stringify(data))
  form.append('photo', photo)
  const response = await fetchWithHandling('/api/patients/me/ktp', {
    method: 'POST',
    signal: AbortSignal.timeout(30000),
    headers: { Accept: 'application/json' },
    body: form,
  })
  await response.json().catch(() => null)
}

export async function apiListDoctorPatients(signal?: AbortSignal): Promise<DoctorPatientSummary[]> {
  const response = await fetchWithHandling('/api/doctor/patients', {
    method: 'GET',
    signal: signal ?? AbortSignal.timeout(15000),
    headers: { Accept: 'application/json' },
  })
  const data = await response.json()
  if (!data || !Array.isArray(data.patients)) {
    throw new ApiError(500, 'Respons daftar pasien tidak sesuai.')
  }
  return data.patients as DoctorPatientSummary[]
}

export async function apiListDoctorPatientFiles(
  patientProfileId: number,
  signal?: AbortSignal,
): Promise<ApiFileRecord[]> {
  const response = await fetchWithHandling(
    `/api/doctor/patients/${encodeURIComponent(patientProfileId)}/files`,
    {
      method: 'GET',
      signal: signal ?? AbortSignal.timeout(15000),
      headers: { Accept: 'application/json' },
    },
  )
  const data = await response.json()
  if (!data || !Array.isArray(data.files)) {
    throw new ApiError(500, 'Respons daftar berkas pasien tidak sesuai.')
  }
  return data.files as ApiFileRecord[]
}

export async function apiUploadDoctorPatientFile(
  patientProfileId: number,
  file: File,
): Promise<ApiFileRecord> {
  const form = new FormData()
  form.append('file', file)
  const response = await fetchWithHandling(
    `/api/doctor/patients/${encodeURIComponent(patientProfileId)}/files`,
    {
      method: 'POST',
      signal: AbortSignal.timeout(30000),
      headers: { Accept: 'application/json' },
      body: form,
    },
  )
  const data = await response.json()
  if (!data || !data.file || typeof data.file.id !== 'number') {
    throw new ApiError(500, 'Respons unggah berkas pasien tidak sesuai.')
  }
  return data.file as ApiFileRecord
}
