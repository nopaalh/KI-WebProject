<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import { ApiError, apiCreatePatientKTP, apiCreatePatientProfile, apiGetPatientKTP, apiGetPatientKTPPhoto, apiGetPatientProfile, type CreatePatientKTPPayload, type PatientKTPData, type PatientProfileData } from './api'

  let { user, onUnauthorized }: { user: { name: string; username: string }; onUnauthorized?: () => void } = $props()
  type ProfileDraft = { age?: number; weight_kg?: number; height_cm?: number; nik: string; bpjs_number: string }
  let profile = $state<PatientProfileData | null>(null)
  let draft = $state<ProfileDraft>({ nik: '', bpjs_number: '' })
  let loading = $state(true)
  let submitting = $state(false)
  let showNumbers = $state(false)
  let notice = $state('')
  let ktp = $state<PatientKTPData | null>(null)
  let ktpPhotoUrl = $state('')
  let ktpLoading = $state(false)
  let ktpSubmitting = $state(false)
  let ktpNotice = $state('')
  let selectedPhoto = $state<File | null>(null)
  let photoInput = $state<HTMLInputElement>()
  let ktpDraft = $state<CreatePatientKTPPayload>({ full_name: '', birth_place: '', birth_date: '', gender: 'Laki-laki', blood_type: 'O', address: '', rt: '001', rw: '001', village: '', district: '', religion: '', marital_status: 'Belum Kawin', occupation: '', citizenship: 'WNI' })

  onDestroy(() => { if (ktpPhotoUrl) URL.revokeObjectURL(ktpPhotoUrl) })

  onMount(() => {
    const controller = new AbortController()
    void loadProfile(controller.signal)
    return () => controller.abort()
  })

  async function loadProfile(signal?: AbortSignal) {
    loading = true
    try {
      profile = await apiGetPatientProfile(signal)
      if (!ktpDraft.full_name) ktpDraft.full_name = user.name
      await loadKtp(signal)
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      if (err instanceof ApiError && err.status === 401) { onUnauthorized?.(); return }
      if (err instanceof ApiError && err.status === 404) { profile = null; ktp = null }
      else notice = err instanceof ApiError && err.isNetworkError ? 'Tidak dapat terhubung ke layanan profil.' : 'Gagal memuat profil pasien.'
    } finally {
      loading = false
    }
  }

  async function loadKtp(signal?: AbortSignal) {
    ktpLoading = true
    try {
      ktp = await apiGetPatientKTP(signal)
      if (ktpPhotoUrl) { URL.revokeObjectURL(ktpPhotoUrl); ktpPhotoUrl = '' }
      const blob = await apiGetPatientKTPPhoto(signal)
      ktpPhotoUrl = URL.createObjectURL(blob)
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      if (err instanceof ApiError && err.status === 401) { onUnauthorized?.(); return }
      if (err instanceof ApiError && err.status === 404) { ktp = null }
      else ktpNotice = 'Gagal memuat data atau foto KTP dari server.'
    } finally {
      ktpLoading = false
    }
  }

  async function saveKtp(event: SubmitEvent) {
    event.preventDefault()
    if (ktpSubmitting || !profile || ktp || !selectedPhoto) return
    if (selectedPhoto.size > 8 * 1024 * 1024) { ktpNotice = 'Ukuran foto KTP maksimal 8 MiB.'; return }
    ktpSubmitting = true
    ktpNotice = ''
    try {
      await apiCreatePatientKTP({ ...ktpDraft, full_name: ktpDraft.full_name.trim(), birth_place: ktpDraft.birth_place.trim(), birth_date: ktpDraft.birth_date.trim(), address: ktpDraft.address.trim(), rt: ktpDraft.rt.trim(), rw: ktpDraft.rw.trim(), village: ktpDraft.village.trim(), district: ktpDraft.district.trim(), religion: ktpDraft.religion.trim(), occupation: ktpDraft.occupation.trim() }, selectedPhoto)
      selectedPhoto = null
      if (photoInput) photoInput.value = ''
      ktpNotice = 'Data dan foto KTP berhasil disimpan.'
      await loadKtp()
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) { onUnauthorized?.(); return }
      if (err instanceof ApiError && err.status === 409) { ktpNotice = 'KTP sudah terdaftar.'; await loadKtp(); return }
      ktpNotice = err instanceof ApiError && err.status === 400 ? 'Pastikan seluruh kolom KTP terisi dan foto berformat PNG/JPEG valid.' : 'Gagal menyimpan data KTP.'
    } finally {
      ktpSubmitting = false
    }
  }

  async function save(event: SubmitEvent) {
    event.preventDefault()
    if (submitting || profile) return
    if (draft.age === undefined || draft.weight_kg === undefined || draft.height_cm === undefined) {
      notice = 'Lengkapi semua kolom profil pasien.'
      return
    }
    submitting = true
    notice = ''
    try {
      await apiCreatePatientProfile({ age: Number(draft.age), weight_kg: Number(draft.weight_kg), height_cm: Number(draft.height_cm), nik: draft.nik.trim(), bpjs_number: draft.bpjs_number.trim() })
      notice = 'Profil pasien berhasil dibuat.'
      await loadProfile()
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) { onUnauthorized?.(); return }
      if (err instanceof ApiError && err.status === 409) { notice = 'Profil sudah terdaftar.'; await loadProfile(); return }
      notice = err instanceof ApiError && err.status === 400 ? 'Periksa kembali format NIK (16 digit), BPJS (13 digit), dan ukuran tubuh.' : 'Gagal menyimpan profil pasien.'
    } finally {
      submitting = false
    }
  }
</script>

<main class="patient-page">
  <header class="patient-heading"><h1 class="patient-title" tabindex="-1">Profil pasien</h1><p class="patient-subtitle">Informasi diri untuk pendaftaran dan administrasi perawatan.</p></header>
  <div class="profile-layout">
    <aside class="patient-card identity">
      <img src="/images/portal-mark.svg" width="38" height="38" alt="" />
      <h2>{user.name}</h2><p>@{user.username}</p>
      <dl><div><dt>Jenis akun</dt><dd>Pasien</dd></div><div><dt>Profil pasien</dt><dd>{loading ? 'Memeriksa…' : profile ? 'Sudah terdaftar' : 'Belum dilengkapi'}</dd></div></dl>
      <p class="patient-note">Nama dan username mengikuti identitas akun Anda.</p>
    </aside>

    <section class="patient-card profile-card" aria-labelledby="personal-title">
      <div class="section-heading"><div><h2 id="personal-title">Data diri</h2><p>{profile ? 'Data profil pasien yang tersimpan pada akun Anda.' : 'Lengkapi data sesuai identitas Anda.'}</p></div></div>
      {#if loading}
        <p class="patient-note" role="status">Memuat profil pasien…</p>
      {:else if !profile}
        <form onsubmit={save} aria-busy={submitting}>
          <fieldset class="measurements" disabled={submitting}><legend>Informasi dasar</legend>
            <div><label for="patient-age">Umur</label><div class="unit-field"><input id="patient-age" name="age" type="number" min="0" max="150" step="1" bind:value={draft.age} placeholder="Contoh: 21" required /><span>tahun</span></div></div>
            <div><label for="patient-weight">Berat badan</label><div class="unit-field"><input id="patient-weight" name="weight_kg" type="number" min="0.01" max="9999.99" step="0.01" bind:value={draft.weight_kg} placeholder="Contoh: 65,5" required /><span>kg</span></div></div>
            <div><label for="patient-height">Tinggi badan</label><div class="unit-field"><input id="patient-height" name="height_cm" type="number" min="0.01" max="9999.99" step="0.01" bind:value={draft.height_cm} placeholder="Contoh: 170" required /><span>cm</span></div></div>
          </fieldset>
          <fieldset class="identity-fields" disabled={submitting}><legend>Identitas pasien</legend>
            <div><label for="patient-nik">NIK</label><input id="patient-nik" name="nik" type="text" inputmode="numeric" pattern={'[0-9]{16}'} minlength="16" maxlength="16" bind:value={draft.nik} autocomplete="off" placeholder="16 digit NIK" aria-describedby="nik-help" required /><p id="nik-help" class="field-help">Nomor yang tercantum pada kartu identitas.</p></div>
            <div><label for="patient-bpjs">Nomor BPJS</label><input id="patient-bpjs" name="bpjs_number" type="text" inputmode="numeric" pattern={'[0-9]{13}'} minlength="13" maxlength="13" bind:value={draft.bpjs_number} autocomplete="off" placeholder="13 digit nomor BPJS" required /><p class="field-help">Gunakan nomor tanpa spasi atau tanda baca.</p></div>
          </fieldset>
          <div class="form-actions"><button class="patient-button" type="submit" disabled={submitting}>{submitting ? 'Menyimpan…' : 'Simpan profil'}</button></div>
          <p class="patient-note">Pembaruan atau penghapusan profil belum tersedia pada layanan saat ini.</p>
        </form>
      {:else}
        <dl class="profile-values"><div><dt>Umur</dt><dd>{profile.age} tahun</dd></div><div><dt>Berat badan</dt><dd>{profile.weight_kg} kg</dd></div><div><dt>Tinggi badan</dt><dd>{profile.height_cm} cm</dd></div></dl>
        <div class="identity-summary"><div class="numbers-heading"><h3>Identitas pasien</h3><button class="text-button" type="button" aria-pressed={showNumbers} onclick={() => showNumbers = !showNumbers}>{showNumbers ? 'Sembunyikan nomor' : 'Tampilkan nomor'}</button></div><dl><div><dt>NIK</dt><dd>{showNumbers ? profile.nik : '•••• •••• •••• ' + profile.nik.slice(-4)}</dd></div><div><dt>Nomor BPJS</dt><dd>{showNumbers ? profile.bpjs_number : '•••• ••••• ' + profile.bpjs_number.slice(-4)}</dd></div></dl></div>
        <div class="ktp-section">
          <div class="numbers-heading"><h3>KTP pasien</h3></div>
          {#if ktpLoading}
            <p class="patient-note">Memuat data KTP…</p>
          {:else if ktp}
            <dl class="ktp-grid">
              <div><dt>Nama KTP</dt><dd>{ktp.full_name}</dd></div><div><dt>Tempat/Tgl lahir</dt><dd>{ktp.birth_place}, {ktp.birth_date}</dd></div>
              <div><dt>Jenis kelamin / Gol. darah</dt><dd>{ktp.gender} / {ktp.blood_type}</dd></div><div><dt>Agama / Perkawinan</dt><dd>{ktp.religion} · {ktp.marital_status}</dd></div>
              <div><dt>Alamat</dt><dd>{ktp.address}, RT {ktp.rt}/RW {ktp.rw}, {ktp.village}, {ktp.district}</dd></div><div><dt>Pekerjaan / Warga</dt><dd>{ktp.occupation} ({ktp.citizenship})</dd></div>
            </dl>
            {#if ktpPhotoUrl}<img class="ktp-photo" src={ktpPhotoUrl} alt="Foto KTP pasien" />{/if}
          {:else}
            <form class="ktp-form" onsubmit={saveKtp} aria-busy={ktpSubmitting}>
              <fieldset class="identity-fields" disabled={ktpSubmitting}><legend>Lengkapi data & foto KTP (NIK mengikuti profil)</legend>
                <div><label for="ktp-name">Nama lengkap</label><input id="ktp-name" bind:value={ktpDraft.full_name} required /></div>
                <div><label for="ktp-place">Tempat lahir</label><input id="ktp-place" bind:value={ktpDraft.birth_place} required /></div>
                <div><label for="ktp-date">Tanggal lahir</label><input id="ktp-date" type="date" bind:value={ktpDraft.birth_date} required /></div>
                <div><label for="ktp-gender">Jenis kelamin</label><select id="ktp-gender" bind:value={ktpDraft.gender}><option value="Laki-laki">Laki-laki</option><option value="Perempuan">Perempuan</option></select></div>
                <div><label for="ktp-blood">Golongan darah</label><select id="ktp-blood" bind:value={ktpDraft.blood_type}><option value="A">A</option><option value="B">B</option><option value="AB">AB</option><option value="O">O</option><option value="-">-</option></select></div>
                <div><label for="ktp-religion">Agama</label><input id="ktp-religion" bind:value={ktpDraft.religion} required /></div>
                <div><label for="ktp-address">Alamat</label><input id="ktp-address" bind:value={ktpDraft.address} required /></div>
                <div><label for="ktp-rt">RT</label><input id="ktp-rt" bind:value={ktpDraft.rt} maxlength="3" required /></div>
                <div><label for="ktp-rw">RW</label><input id="ktp-rw" bind:value={ktpDraft.rw} maxlength="3" required /></div>
                <div><label for="ktp-village">Kelurahan/Desa</label><input id="ktp-village" bind:value={ktpDraft.village} required /></div>
                <div><label for="ktp-district">Kecamatan</label><input id="ktp-district" bind:value={ktpDraft.district} required /></div>
                <div><label for="ktp-marital">Status perkawinan</label><select id="ktp-marital" bind:value={ktpDraft.marital_status}><option value="Belum Kawin">Belum Kawin</option><option value="Kawin">Kawin</option><option value="Cerai Hidup">Cerai Hidup</option><option value="Cerai Mati">Cerai Mati</option></select></div>
                <div><label for="ktp-job">Pekerjaan</label><input id="ktp-job" bind:value={ktpDraft.occupation} required /></div>
                <div><label for="ktp-citizen">Kewarganegaraan</label><select id="ktp-citizen" bind:value={ktpDraft.citizenship}><option value="WNI">WNI</option><option value="WNA">WNA</option></select></div>
                <div><label for="ktp-photo">Foto KTP (PNG/JPEG, maks 8 MiB)</label><input id="ktp-photo" type="file" accept="image/png,image/jpeg" bind:this={photoInput} onchange={e => selectedPhoto = (e.currentTarget as HTMLInputElement).files?.[0] ?? null} required /></div>
              </fieldset>
              <div class="form-actions"><button class="patient-button" type="submit" disabled={ktpSubmitting}>{ktpSubmitting ? 'Menyimpan KTP…' : 'Simpan KTP'}</button></div>
            </form>
          {/if}
          {#if ktpNotice}<p class="patient-message" role="status">{ktpNotice}</p>{/if}
        </div>
      {/if}
      <p class="patient-message" role="status">{notice}</p>
    </section>
  </div>
</main>

<style>
  .profile-layout { display: grid; grid-template-columns: 280px minmax(0, 1fr); gap: 24px; align-items: start; }
  .identity { padding: 28px; }
  .identity img { object-fit: contain; margin-bottom: 22px; }
  .identity h2 { font-size: 24px; letter-spacing: -.6px; overflow-wrap: anywhere; }
  .identity > p { font-size: 12px; overflow-wrap: anywhere; }
  dl { margin: 28px 0 0; }
  dl > div + div { margin-top: 22px; }
  dt { font-size: 12px; margin-bottom: 7px; }
  dd { margin: 0; font-size: 14px; font-weight: 500; overflow-wrap: anywhere; }
  .identity .patient-note { border-top: 1px solid #e1e6ec; padding-top: 22px; margin-top: 26px; }
  .profile-card { padding: 32px; }
  .section-heading { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 18px; margin-bottom: 32px; }
  .section-heading h2 { font-size: 21px; letter-spacing: -.6px; }
  .section-heading p { font-size: 12px; margin: 8px 0 0; }
  fieldset { display: grid; gap: 20px; }
  legend { padding: 0; font-size: 13px; font-weight: 600; margin-bottom: 20px; }
  .measurements { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .identity-fields { margin-top: 32px; padding-top: 26px; border-top: 1px solid #e1e6ec; grid-template-columns: repeat(2, minmax(0, 1fr)); }
  label { font-size: 12px; }
  input, select { display: block; width: 100%; height: 52px; padding: 0 14px; font-size: 14px; border: 1px solid #d2d2d7; border-radius: 12px; background: #fff; font: inherit; }
  input[type="file"] { padding: 11px 12px; height: auto; font-size: 12px; }
  .ktp-section { margin-top: 32px; padding-top: 26px; border-top: 1px solid #e1e6ec; }
  .ktp-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; }
  .ktp-grid > div + div { margin-top: 0; }
  .ktp-photo { display: block; max-width: 320px; width: 100%; margin-top: 20px; border: 1px solid #d1d8e0; border-radius: 12px; }
  input::placeholder { font-size: 13px; }
  .unit-field { position: relative; }
  .unit-field input { padding-right: 52px; }
  .unit-field > span { position: absolute; right: 13px; top: 17px; font-size: 11px; pointer-events: none; }
  .field-help { font-size: 11px; margin: 9px 0 0; line-height: 1.6; }
  .form-actions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 32px; }
  .profile-values { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 20px; }
  .profile-values > div + div { margin-top: 0; }
  .profile-values dd { font-size: 24px; letter-spacing: -.6px; }
  .identity-summary { margin-top: 32px; padding-top: 26px; border-top: 1px solid #e1e6ec; }
  .numbers-heading { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 14px; }
  h3 { margin: 0; font-size: 14px; }
  .identity-summary dl { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px; font-variant-numeric: tabular-nums; }
  .identity-summary dl > div + div { margin-top: 0; }
  @media (max-width: 900px) { .profile-layout { grid-template-columns: minmax(0, 1fr); } .identity dl { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px; } .identity dl > div + div { margin-top: 0; } }
  @media (max-width: 540px) { .profile-card, .identity { padding: 24px; } .measurements { grid-template-columns: minmax(0, 1fr); } .identity-fields { grid-template-columns: minmax(0, 1fr); } .profile-values { grid-template-columns: minmax(0, 1fr); gap: 22px; } .identity-summary dl { grid-template-columns: minmax(0, 1fr); } }
</style>
