export type DocumentCategory = 'Semua' | 'Dokumen' | 'Foto' | 'Video'
export type PatientDocument = { name: string; kind: string; category: DocumentCategory; size: number; selected?: boolean }

export const exampleDocuments: PatientDocument[] = [
  { name: 'Kartu identitas.pdf', kind: 'PDF', category: 'Dokumen', size: 245760 },
  { name: 'Formulir pendaftaran.docx', kind: 'DOCX', category: 'Dokumen', size: 86016 },
  { name: 'Foto kartu BPJS.png', kind: 'PNG', category: 'Foto', size: 1258291 },
  { name: 'Video keluhan.mp4', kind: 'MP4', category: 'Video', size: 5662310 },
]
