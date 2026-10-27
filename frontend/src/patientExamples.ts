export type DocumentCategory = 'Semua' | 'Dokumen' | 'Foto' | 'Video'
export type PatientDocument = { name: string; kind: string; category: DocumentCategory; size: number; selected?: boolean }

export const exampleDocuments: PatientDocument[] = [
  { name: 'Kartu identitas.pdf', kind: 'PDF', category: 'Dokumen', size: 245760 },
  { name: 'Formulir pendaftaran.docx', kind: 'DOCX', category: 'Dokumen', size: 86016 },
  { name: 'Foto kartu BPJS.png', kind: 'PNG', category: 'Foto', size: 1258291 },
  { name: 'Video keluhan.mp4', kind: 'MP4', category: 'Video', size: 5662310 },
]

export type MedicalVersion = {
  version: number
  date: string
  author: string
  status: 'Draf' | 'Final'
  summary: string
  note: string
}

export type MedicalRecord = {
  id: number
  title: string
  department: string
  reference: string
  versions: MedicalVersion[]
}

export const exampleMedicalRecords: MedicalRecord[] = [
  {
    id: 1,
    title: 'Pemeriksaan MRI',
    department: 'Radiologi',
    reference: 'RM-2026-001',
    versions: [
      { version: 3, date: '25 Oktober 2026, 10.20', author: 'dr. Nadira Putri', status: 'Draf', summary: 'Laporan pemeriksaan sedang ditinjau.', note: 'Citra pemeriksaan telah diterima. Dokter sedang melengkapi laporan dan memeriksa catatan sebelum hasil akhir diterbitkan.' },
      { version: 2, date: '24 Oktober 2026, 15.40', author: 'dr. Nadira Putri', status: 'Draf', summary: 'Pemeriksaan selesai dilakukan.', note: 'Pemeriksaan telah selesai. Citra sedang dipersiapkan untuk peninjauan dokter. Hasil pemeriksaan belum tersedia pada catatan ini.' },
      { version: 1, date: '24 Oktober 2026, 08.15', author: 'dr. Nadira Putri', status: 'Draf', summary: 'Catatan pemeriksaan dibuat.', note: 'Permintaan pemeriksaan telah dicatat oleh bagian radiologi. Catatan ini akan diperbarui setelah pemeriksaan dilakukan.' },
    ],
  },
  {
    id: 2,
    title: 'Pemeriksaan laboratorium',
    department: 'Laboratorium',
    reference: 'RM-2026-002',
    versions: [
      { version: 2, date: '22 Oktober 2026, 14.10', author: 'dr. Arga Pratama', status: 'Final', summary: 'Laporan pemeriksaan telah ditinjau.', note: 'Laporan telah diperiksa dan ditandai final oleh dokter. Contoh ini hanya memperlihatkan susunan catatan, tanpa nilai pemeriksaan atau kesimpulan klinis.' },
      { version: 1, date: '22 Oktober 2026, 09.30', author: 'dr. Arga Pratama', status: 'Draf', summary: 'Berkas pemeriksaan diterima.', note: 'Berkas pemeriksaan telah diterima oleh laboratorium. Laporan masih menunggu peninjauan dokter.' },
    ],
  },
  {
    id: 3,
    title: 'Kunjungan rawat jalan',
    department: 'Poliklinik umum',
    reference: 'RM-2026-003',
    versions: [
      { version: 1, date: '18 Oktober 2026, 11.05', author: 'dr. Nadira Putri', status: 'Final', summary: 'Catatan kunjungan telah selesai.', note: 'Dokter telah menyelesaikan pencatatan kunjungan. Contoh ini tidak memuat diagnosis, resep, atau anjuran perawatan.' },
    ],
  },
]
