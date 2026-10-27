import { exampleDocuments, exampleMedicalRecords, type MedicalRecord, type PatientDocument } from './patientExamples'

export type DoctorDocument = PatientDocument & { localFile?: File; source: 'Pasien' | 'Rumah sakit' }
export type DoctorPatient = {
  id: number
  name: string
  reference: string
  portrait: 'male' | 'female'
  age: number
  weight: number
  height: number
  records: MedicalRecord[]
  documents: DoctorDocument[]
}

export const exampleDoctorPatients: DoctorPatient[] = [
  { id: 1, name: 'Aditya Pratama', portrait: 'male', reference: 'PS-001', age: 27, weight: 68, height: 174, records: structuredClone(exampleMedicalRecords), documents: exampleDocuments.map(item => ({ ...item, source: 'Pasien' })) },
  { id: 2, name: 'Nadia Safitri', portrait: 'female', reference: 'PS-002', age: 31, weight: 55, height: 162, records: structuredClone(exampleMedicalRecords.slice(1, 2)), documents: exampleDocuments.slice(0, 2).map(item => ({ ...item, source: 'Pasien' })) },
  { id: 3, name: 'Bima Saputra', portrait: 'male', reference: 'PS-003', age: 24, weight: 72, height: 178, records: [], documents: [] },
]
