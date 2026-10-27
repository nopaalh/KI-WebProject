<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import {
    ApiError,
    apiCreatePatientKTP,
    apiCreatePatientProfile,
    apiGetPatientKTP,
    apiGetPatientKTPPhoto,
    apiGetPatientProfile,
    type CreatePatientKTPPayload,
    type PatientKTPData,
    type PatientProfileData,
  } from './api'

  let {
    user,
    onUnauthorized,
  }: {
    user: { name: string; username: string }
    onUnauthorized?: () => void
  } = $props()

  let loadingProfile = $state(true)
  let profileError = $state('')
  let profile = $state<PatientProfileData | null>(null)
  let submittingProfile = $state(false)
  let profileNotice = $state('')
  let profileNoticeError = $state(false)
  let showNumbers = $state(false)

  let draftProfile = $state<{
    age: string
    weight_kg: string
    height_cm: string
    nik: string
    bpjs_number: string
  }>({
    age: '',
    weight_kg: '',
    height_cm: '',
    nik: '',
    bpjs_number: '',
  })

  let loadingKtp = $state(false)
  let ktpError = $state('')
  let ktp = $state<PatientKTPData | null>(null)
  let ktpPhotoUrl = $state('')
  let loadingKtpPhoto = $state(false)
  let ktpPhotoError = $state('')
  let submittingKtp = $state(false)
  let ktpNotice = $state('')
  let ktpNoticeError = $state(false)
  let selectedPhoto = $state<File | null>(null)
  let photoInput = $state<HTMLInputElement>()

  let draftKtp = $state<CreatePatientKTPPayload>({
    full_name: '',
    birth_place: '',
    birth_date: '',
    gender: 'Laki-laki',
    blood_type: 'O',
    address: '',
    rt: '001',
    rw: '001',
    village: '',
    district: '',
    religion: '',
    marital_status: 'Belum Kawin',
    occupation: '',
    citizenship: 'WNI',
  })

  function revokeKtpPhoto() {
    if (ktpPhotoUrl) {
      URL.revokeObjectURL(ktpPhotoUrl)
      ktpPhotoUrl = ''
    }
  }

  onDestroy(() => {
    revokeKtpPhoto()
  })

  onMount(() => {
    const controller = new AbortController()
    void loadProfileAndKtp(controller.signal)
    return () => controller.abort()
  })

  async function loadProfileAndKtp(signal?: AbortSignal) {
    loadingProfile = true
    profileError = ''
    try {
      const loaded = await apiGetPatientProfile(signal)
      profile = loaded
      if (!draftKtp.full_name) {
        draftKtp.full_name = user.name
      }
      await loadKtp(signal)
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      if (err instanceof ApiError) {
        if (err.status === 401) {
          onUnauthorized?.()
          return
        }
        if (err.status === 404) {
          profile = null
          ktp = null
          revokeKtpPhoto()
        } else if (err.status === 403) {
          profileError = 'Hanya akun pasien yang dapat mengakses profil pasien sendiri.'
        } else if (err.isNetworkError) {
          profileError = 'Tidak dapat terhubung ke server untuk memuat profil pasien.'
        } else {
          profileError = 'Gagal memuat profil pasien dari server.'
        }
      } else {
        profileError = 'Terjadi kesalahan saat memuat profil pasien.'
      }
    } finally {
      loadingProfile = false
    }
  }

  async function loadKtp(signal?: AbortSignal) {
    loadingKtp = true
    ktpError = ''
    try {
      const loadedKtp = await apiGetPatientKTP(signal)
      ktp = loadedKtp
      await loadKtpPhoto(signal)
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      if (err instanceof ApiError) {
        if (err.status === 401) {
          onUnauthorized?.()
          return
        }
        if (err.status === 404) {
          ktp = null
          revokeKtpPhoto()
        } else if (err.status === 403) {
          ktpError = 'Hanya akun pasien pemilik yang dapat membaca data KTP.'
        } else if (err.isNetworkError) {
          ktpError = 'Tidak dapat terhubung ke server untuk memuat data KTP.'
        } else {
          ktpError = 'Gagal memuat data KTP dari server.'
        }
      } else {
        ktpError = 'Terjadi kesalahan saat memuat data KTP.'
      }
    } finally {
      loadingKtp = false
    }
  }

  async function loadKtpPhoto(signal?: AbortSignal) {
    loadingKtpPhoto = true
    ktpPhotoError = ''
    revokeKtpPhoto()
    try {
      const blob = await apiGetPatientKTPPhoto(signal)
      ktpPhotoUrl = URL.createObjectURL(blob)
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      if (err instanceof ApiError) {
        if (err.status === 401) {
          onUnauthorized?.()
          return
        }
        if (err.status === 404) {
          ktpPhotoError = 'Foto KTP belum ditemukan.'
        } else if (err.isNetworkError) {
          ktpPhotoError = 'Tidak dapat terhubung ke server untuk memuat foto KTP.'
        } else {
          ktpPhotoError = 'Gagal memuat foto KTP dari server.'
        }
      } else {
        ktpPhotoError = 'Terjadi kesalahan saat memuat foto KTP.'
      }
    } finally {
      loadingKtpPhoto = false
    }
  }

  async function submitProfile(event: SubmitEvent) {
    event.preventDefault()
    if (submittingProfile || profile) return
    profileNotice = ''
    profileNoticeError = false

    const ageNum = Number(draftProfile.age)
    const weightNum = Number(draftProfile.weight_kg)
    const heightNum = Number(draftProfile.height_cm)
    const nikClean = draftProfile.nik.trim()
    const bpjsClean = draftProfile.bpjs_number.trim()

    if (!Number.isInteger(ageNum) || ageNum < 0 || ageNum > 150) {
      profileNoticeError = true
      profileNotice = 'Umur harus berupa bilangan bulat antara 0 sampai 150.'
      return
    }
    if (!Number.isFinite(weightNum) || weightNum <= 0 || weightNum > 9999.99) {
      profileNoticeError = true
      profileNotice = 'Berat badan harus positif (maksimal 9999,99 kg).'
      return
    }
    if (!Number.isFinite(heightNum) || heightNum <= 0 || heightNum > 9999.99) {
      profileNoticeError = true
      profileNotice = 'Tinggi badan harus positif (maksimal 9999,99 cm).'
      return
    }
    if (!/^[0-9]{16}$/.test(nikClean)) {
      profileNoticeError = true
      profileNotice = 'NIK wajib terdiri dari tepat 16 digit angka.'
      return
    }
    if (!/^[0-9]{13}$/.test(bpjsClean)) {
      profileNoticeError = true
      profileNotice = 'Nomor BPJS wajib terdiri dari tepat 13 digit angka.'
      return
    }

    submittingProfile = true
    try {
      await apiCreatePatientProfile({
        age: ageNum,
        weight_kg: Math.round(weightNum * 100) / 100,
        height_cm: Math.round(heightNum * 100) / 100,
        nik: nikClean,
        bpjs_number: bpjsClean,
      })
      profileNotice = 'Profil pasien berhasil dibuat dan disimpan di server.'
      await loadProfileAndKtp()
    } catch (err) {
      profileNoticeError = true
      if (err instanceof ApiError) {
        if (err.status === 401) {
          onUnauthorized?.()
          return
        }
        if (err.status === 409) {
          profileNotice = 'Profil pasien sudah terdaftar untuk akun ini. Memuat data profil…'
          await loadProfileAndKtp()
          return
        }
        if (err.status === 403) {
          profileNotice = 'Hanya akun pasien yang dapat membuat profil pasien.'
        } else if (err.status === 400) {
          profileNotice = 'Data profil belum sesuai format yang disyaratkan server.'
        } else if (err.isNetworkError) {
          profileNotice = 'Tidak dapat terhubung ke server saat menyimpan profil.'
        } else {
          profileNotice = 'Gagal menyimpan profil pasien di server.'
        }
      } else {
        profileNotice = 'Terjadi kesalahan saat menyimpan profil.'
      }
    } finally {
      submittingProfile = false
    }
  }

  function handlePhotoChange(event: Event) {
    const file = (event.currentTarget as HTMLInputElement).files?.[0]
    if (!file) {
      selectedPhoto = null
      return
    }
    ktpNotice = ''
    ktpNoticeError = false
    if (file.size > 8 * 1024 * 1024) {
      ktpNoticeError = true
      ktpNotice = 'Ukuran foto KTP maksimal 8 MiB.'
      if (photoInput) photoInput.value = ''
      selectedPhoto = null
      return
    }
    selectedPhoto = file
  }

  async function submitKtp(event: SubmitEvent) {
    event.preventDefault()
    if (submittingKtp || !profile || ktp) return
    ktpNotice = ''
    ktpNoticeError = false

    if (!selectedPhoto) {
      ktpNoticeError = true
      ktpNotice = 'Pilih file foto KTP (PNG atau JPEG, maksimal 8 MiB).'
      return
    }

    const payload: CreatePatientKTPPayload = {
      full_name: draftKtp.full_name.trim(),
      birth_place: draftKtp.birth_place.trim(),
      birth_date: draftKtp.birth_date.trim(),
      gender: draftKtp.gender,
      blood_type: draftKtp.blood_type,
      address: draftKtp.address.trim(),
      rt: draftKtp.rt.trim(),
      rw: draftKtp.rw.trim(),
      village: draftKtp.village.trim(),
      district: draftKtp.district.trim(),
      religion: draftKtp.religion.trim(),
      marital_status: draftKtp.marital_status,
      occupation: draftKtp.occupation.trim(),
      citizenship: draftKtp.citizenship,
    }

    if (
      !payload.full_name ||
      !payload.birth_place ||
      !payload.birth_date ||
      !payload.address ||
      !payload.rt ||
      !payload.rw ||
      !payload.village ||
      !payload.district ||
      !payload.religion ||
      !payload.occupation
    ) {
      ktpNoticeError = true
      ktpNotice = 'Lengkapi seluruh kolom data KTP.'
      return
    }

    if (!/^(?:[0-9]{1,3}|-)$/.test(payload.rt) || !/^(?:[0-9]{1,3}|-)$/.test(payload.rw)) {
      ktpNoticeError = true
      ktpNotice = 'RT dan RW harus berupa 1–3 digit angka atau tanda "-".'
      return
    }

    submittingKtp = true
    try {
      await apiCreatePatientKTP(payload, selectedPhoto)
      selectedPhoto = null
      if (photoInput) photoInput.value = ''
      ktpNotice = 'Data dan foto KTP berhasil disimpan di server.'
      await loadKtp()
    } catch (err) {
      ktpNoticeError = true
      if (err instanceof ApiError) {
        if (err.status === 401) {
          onUnauthorized?.()
          return
        }
        if (err.status === 409) {
          ktpNotice = 'Data KTP sudah terdaftar untuk profil ini. Memuat data KTP…'
          await loadKtp()
          return
        }
        if (err.status === 404) {
          ktpNotice = 'Profil pasien belum ditemukan. Buat profil pasien terlebih dahulu.'
        } else if (err.status === 413) {
          ktpNotice = 'Ukuran foto atau data KTP melebihi batas maksimum (8 MiB).'
        } else if (err.status === 400) {
          ktpNotice = 'Data KTP atau format gambar (wajib PNG/JPEG valid) tidak sesuai ketentuan.'
        } else if (err.status === 403) {
          ktpNotice = 'Hanya akun pasien pemilik yang dapat menyimpan KTP.'
        } else if (err.isNetworkError) {
          ktpNotice = 'Tidak dapat terhubung ke server saat menyimpan KTP.'
        } else {
          ktpNotice = 'Gagal menyimpan data KTP di server.'
        }
      } else {
        ktpNotice = 'Terjadi kesalahan saat menyimpan KTP.'
      }
    } finally {
      submittingKtp = false
    }
  }
</script>

<main class="patient-page">
  <header class="patient-heading">
    <h1 class="patient-title" tabindex="-1">Profil & KTP pasien</h1>
    <p class="patient-subtitle">Informasi diri dan dokumen identitas untuk pendaftaran serta administrasi perawatan.</p>
  </header>

  <div class="profile-layout">
    <aside class="patient-card identity">
      <img src="/images/portal-mark.svg" width="38" height="38" alt="" />
      <h2>{user.name}</h2>
      <p>@{user.username}</p>
      <dl>
        <div><dt>Jenis akun</dt><dd>Pasien</dd></div>
        <div><dt>Profil pasien</dt><dd>{loadingProfile ? 'Memeriksa…' : profile ? `Tersimpan (#${profile.id})` : 'Belum dibuat'}</dd></div>
        <div><dt>Data KTP</dt><dd>{loadingProfile || loadingKtp ? 'Memeriksa…' : ktp ? `Tersimpan (#${ktp.id})` : 'Belum dilengkapi'}</dd></div>
      </dl>
      <p class="patient-note">Nama dan username di atas adalah identitas akun login (`/api/users/me`), terpisah dari profil medis dan KTP pasien.</p>
    </aside>

    <div class="profile-main-column">
      <section class="patient-card profile-card" aria-labelledby="personal-title">
        <div class="section-heading">
          <div>
            <h2 id="personal-title">Profil pasien</h2>
            <p>{profile ? 'Data profil pasien yang tersimpan pada akun Anda.' : 'Buat profil pasien terlebih dahulu sebelum mengunggah KTP atau berkas lampiran.'}</p>
          </div>
          {#if profileError}
            <button class="patient-button secondary" type="button" onclick={() => loadProfileAndKtp()}>Coba lagi</button>
          {/if}
        </div>

        {#if loadingProfile}
          <p class="patient-note" role="status">Memuat profil pasien dari server…</p>
        {:else if profileError}
          <p class="patient-message error" role="alert">{profileError}</p>
        {:else if !profile}
          <form onsubmit={submitProfile} aria-busy={submittingProfile}>
            <fieldset class="measurements" disabled={submittingProfile}>
              <legend>Informasi dasar</legend>
              <div>
                <label for="patient-age">Umur</label>
                <div class="unit-field">
                  <input id="patient-age" name="age" type="number" min="0" max="150" step="1" bind:value={draftProfile.age} placeholder="Contoh: 21" required />
                  <span>tahun</span>
                </div>
              </div>
              <div>
                <label for="patient-weight">Berat badan</label>
                <div class="unit-field">
                  <input id="patient-weight" name="weight_kg" type="number" min="0.01" max="9999.99" step="0.01" bind:value={draftProfile.weight_kg} placeholder="Contoh: 65.5" required />
                  <span>kg</span>
                </div>
              </div>
              <div>
                <label for="patient-height">Tinggi badan</label>
                <div class="unit-field">
                  <input id="patient-height" name="height_cm" type="number" min="0.01" max="9999.99" step="0.01" bind:value={draftProfile.height_cm} placeholder="Contoh: 170" required />
                  <span>cm</span>
                </div>
              </div>
            </fieldset>

            <fieldset class="identity-fields" disabled={submittingProfile}>
              <legend>Identitas pasien</legend>
              <div>
                <label for="patient-nik">NIK</label>
                <input id="patient-nik" name="nik" type="text" inputmode="numeric" pattern="[0-9]{'{'}16{'}'}" minlength="16" maxlength="16" bind:value={draftProfile.nik} autocomplete="off" placeholder="16 digit NIK" aria-describedby="nik-help" required />
                <p id="nik-help" class="field-help">Tepat 16 digit angka sesuai KTP. NIK ini juga akan digunakan secara otomatis saat pendaftaran KTP.</p>
              </div>
              <div>
                <label for="patient-bpjs">Nomor BPJS</label>
                <input id="patient-bpjs" name="bpjs_number" type="text" inputmode="numeric" pattern="[0-9]{'{'}13{'}'}" minlength="13" maxlength="13" bind:value={draftProfile.bpjs_number} autocomplete="off" placeholder="13 digit nomor BPJS" required />
                <p class="field-help">Tepat 13 digit angka tanpa spasi atau tanda baca.</p>
              </div>
            </fieldset>

            <div class="form-actions">
              <button class="patient-button" type="submit" disabled={submittingProfile}>
                {submittingProfile ? 'Menyimpan profil…' : 'Buat profil pasien'}
              </button>
            </div>
            <p class="patient-note">Perhatian: Endpoint pembaruan atau penghapusan profil belum tersedia di backend. Pastikan data sudah benar sebelum disimpan.</p>
          </form>
        {:else}
          <dl class="profile-values">
            <div><dt>Umur</dt><dd>{profile.age} tahun</dd></div>
            <div><dt>Berat badan</dt><dd>{profile.weight_kg} kg</dd></div>
            <div><dt>Tinggi badan</dt><dd>{profile.height_cm} cm</dd></div>
          </dl>
          <div class="identity-summary">
            <div class="numbers-heading">
              <h3>Nomor identitas</h3>
              <button class="text-button" type="button" aria-pressed={showNumbers} onclick={() => showNumbers = !showNumbers}>
                {showNumbers ? 'Sembunyikan nomor' : 'Tampilkan nomor'}
              </button>
            </div>
            <dl>
              <div><dt>NIK</dt><dd>{showNumbers ? profile.nik : '•••• •••• •••• ' + profile.nik.slice(-4)}</dd></div>
              <div><dt>Nomor BPJS</dt><dd>{showNumbers ? profile.bpjs_number : '•••• ••••• ' + profile.bpjs_number.slice(-4)}</dd></div>
            </dl>
          </div>
          <p class="patient-note">Pembaruan atau penghapusan profil belum didukung oleh layanan saat ini.</p>
        {/if}

        {#if profileNotice}
          <p class="patient-message" class:error={profileNoticeError} role={profileNoticeError ? 'alert' : 'status'}>{profileNotice}</p>
        {/if}
      </section>

      <section class="patient-card profile-card ktp-card" aria-labelledby="ktp-title">
        <div class="section-heading">
          <div>
            <h2 id="ktp-title">KTP pasien</h2>
            <p>
              {#if !profile}
                Selesaikan pembuatan profil pasien terlebih dahulu sebelum mengisi data KTP.
              {:else if ktp}
                Data KTP dan foto yang tersimpan pada profil pasien Anda.
              {:else}
                Lengkapi data KTP dan unggah foto KTP (PNG/JPEG, maksimal 8 MiB).
              {/if}
            </p>
          </div>
          {#if profile && (ktpError || ktpPhotoError)}
            <button class="patient-button secondary" type="button" onclick={() => loadKtp()}>Muat ulang KTP</button>
          {/if}
        </div>

        {#if !profile}
          <p class="patient-note">Data KTP memerlukan NIK dari profil pasien yang sudah terdaftar di backend.</p>
        {:else if loadingKtp}
          <p class="patient-note" role="status">Memuat data KTP dari server…</p>
        {:else if ktpError}
          <p class="patient-message error" role="alert">{ktpError}</p>
        {:else if ktp}
          <div class="ktp-view-grid">
            <dl class="ktp-details">
              <div><dt>NIK (dari profil)</dt><dd>{showNumbers ? ktp.nik : '•••• •••• •••• ' + ktp.nik.slice(-4)}</dd></div>
              <div><dt>Nama lengkap KTP</dt><dd>{ktp.full_name}</dd></div>
              <div><dt>Tempat, tanggal lahir</dt><dd>{ktp.birth_place}, {ktp.birth_date}</dd></div>
              <div><dt>Jenis kelamin</dt><dd>{ktp.gender}</dd></div>
              <div><dt>Golongan darah</dt><dd>{ktp.blood_type}</dd></div>
              <div><dt>Alamat</dt><dd>{ktp.address}</dd></div>
              <div><dt>RT / RW</dt><dd>{ktp.rt} / {ktp.rw}</dd></div>
              <div><dt>Kelurahan / Desa</dt><dd>{ktp.village}</dd></div>
              <div><dt>Kecamatan</dt><dd>{ktp.district}</dd></div>
              <div><dt>Agama</dt><dd>{ktp.religion}</dd></div>
              <div><dt>Status perkawinan</dt><dd>{ktp.marital_status}</dd></div>
              <div><dt>Pekerjaan</dt><dd>{ktp.occupation}</dd></div>
              <div><dt>Kewarganegaraan</dt><dd>{ktp.citizenship}</dd></div>
            </dl>
            <div class="ktp-photo-box">
              <h3>Foto KTP</h3>
              {#if loadingKtpPhoto}
                <p class="patient-note" role="status">Memuat foto KTP…</p>
              {:else if ktpPhotoError}
                <p class="patient-message error" role="alert">{ktpPhotoError}</p>
              {:else if ktpPhotoUrl}
                <img class="ktp-photo" src={ktpPhotoUrl} alt="Foto KTP pasien" />
              {/if}
            </div>
          </div>
          <p class="patient-note">Pembaruan atau penghapusan KTP belum didukung oleh layanan saat ini.</p>
        {:else}
          <form onsubmit={submitKtp} aria-busy={submittingKtp}>
            <fieldset class="ktp-form-grid" disabled={submittingKtp}>
              <legend>Formulir KTP pasien</legend>
              <div class="full-span">
                <label for="ktp-nik-readonly">NIK (diambil dari profil Anda)</label>
                <input id="ktp-nik-readonly" type="text" value={profile.nik} readonly />
              </div>
              <div>
                <label for="ktp-fullname">Nama lengkap sesuai KTP</label>
                <input id="ktp-fullname" type="text" bind:value={draftKtp.full_name} placeholder="Nama lengkap" required />
              </div>
              <div>
                <label for="ktp-birthplace">Tempat lahir</label>
                <input id="ktp-birthplace" type="text" bind:value={draftKtp.birth_place} placeholder="Kota/Kabupaten kelahiran" required />
              </div>
              <div>
                <label for="ktp-birthdate">Tanggal lahir</label>
                <input id="ktp-birthdate" type="date" bind:value={draftKtp.birth_date} required />
              </div>
              <div>
                <label for="ktp-gender">Jenis kelamin</label>
                <select id="ktp-gender" bind:value={draftKtp.gender} required>
                  <option value="Laki-laki">Laki-laki</option>
                  <option value="Perempuan">Perempuan</option>
                </select>
              </div>
              <div>
                <label for="ktp-blood">Golongan darah</label>
                <select id="ktp-blood" bind:value={draftKtp.blood_type} required>
                  <option value="A">A</option>
                  <option value="B">B</option>
                  <option value="AB">AB</option>
                  <option value="O">O</option>
                  <option value="-">- (Tidak diketahui)</option>
                </select>
              </div>
              <div>
                <label for="ktp-religion">Agama</label>
                <input id="ktp-religion" type="text" bind:value={draftKtp.religion} placeholder="Contoh: Islam" required />
              </div>
              <div class="full-span">
                <label for="ktp-address">Alamat</label>
                <input id="ktp-address" type="text" bind:value={draftKtp.address} placeholder="Jalan, nomor rumah, blok" required />
              </div>
              <div>
                <label for="ktp-rt">RT</label>
                <input id="ktp-rt" type="text" bind:value={draftKtp.rt} placeholder="Contoh: 001 atau -" maxlength="3" required />
              </div>
              <div>
                <label for="ktp-rw">RW</label>
                <input id="ktp-rw" type="text" bind:value={draftKtp.rw} placeholder="Contoh: 002 atau -" maxlength="3" required />
              </div>
              <div>
                <label for="ktp-village">Kelurahan / Desa</label>
                <input id="ktp-village" type="text" bind:value={draftKtp.village} placeholder="Nama kelurahan atau desa" required />
              </div>
              <div>
                <label for="ktp-district">Kecamatan</label>
                <input id="ktp-district" type="text" bind:value={draftKtp.district} placeholder="Nama kecamatan" required />
              </div>
              <div>
                <label for="ktp-marital">Status perkawinan</label>
                <select id="ktp-marital" bind:value={draftKtp.marital_status} required>
                  <option value="Belum Kawin">Belum Kawin</option>
                  <option value="Kawin">Kawin</option>
                  <option value="Cerai Hidup">Cerai Hidup</option>
                  <option value="Cerai Mati">Cerai Mati</option>
                </select>
              </div>
              <div>
                <label for="ktp-occupation">Pekerjaan</label>
                <input id="ktp-occupation" type="text" bind:value={draftKtp.occupation} placeholder="Contoh: Pelajar / Karyawan" required />
              </div>
              <div>
                <label for="ktp-citizenship">Kewarganegaraan</label>
                <select id="ktp-citizenship" bind:value={draftKtp.citizenship} required>
                  <option value="WNI">WNI</option>
                  <option value="WNA">WNA</option>
                </select>
              </div>
              <div>
                <label for="ktp-photo">Foto KTP (PNG / JPEG)</label>
                <input id="ktp-photo" type="file" accept="image/png,image/jpeg" bind:this={photoInput} onchange={handlePhotoChange} required />
                <p class="field-help">Maksimal 8 MiB dan 25 juta piksel.</p>
              </div>
            </fieldset>

            <div class="form-actions">
              <button class="patient-button" type="submit" disabled={submittingKtp}>
                {submittingKtp ? 'Menyimpan KTP…' : 'Simpan data & foto KTP'}
              </button>
            </div>
          </form>
        {/if}

        {#if ktpNotice}
          <p class="patient-message" class:error={ktpNoticeError} role={ktpNoticeError ? 'alert' : 'status'}>{ktpNotice}</p>
        {/if}
      </section>
    </div>
  </div>
</main>

<style>
  .profile-layout { display: grid; grid-template-columns: 280px minmax(0, 1fr); gap: 24px; align-items: start; }
  .profile-main-column { display: grid; gap: 24px; min-width: 0; }
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
  .section-heading h2 { font-size: 21px; letter-spacing: -.6px; margin: 0; }
  .section-heading p { font-size: 12px; margin: 8px 0 0; }
  fieldset { display: grid; gap: 20px; }
  legend { padding: 0; font-size: 13px; font-weight: 600; margin-bottom: 20px; }
  .measurements { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .identity-fields { margin-top: 32px; padding-top: 26px; border-top: 1px solid #e1e6ec; grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .ktp-form-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .ktp-form-grid .full-span { grid-column: 1 / -1; }
  label { font-size: 12px; }
  input, select { display: block; width: 100%; height: 52px; padding: 0 14px; font-size: 14px; border: 1px solid #d2d2d7; border-radius: 12px; background: #fff; font-family: inherit; color: inherit; }
  input[readonly] { background: #f4f6f9; color: #4a5568; }
  input[type="file"] { padding: 11px 12px; height: auto; font-size: 12px; }
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
  .ktp-view-grid { display: grid; grid-template-columns: 1.3fr 1fr; gap: 28px; align-items: start; }
  .ktp-details { margin: 0; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; }
  .ktp-details > div + div { margin-top: 0; }
  .ktp-photo-box { padding: 18px; border: 1px solid #dce3ec; border-radius: 16px; background: #f8fafd; }
  .ktp-photo-box h3 { margin-bottom: 14px; }
  .ktp-photo { display: block; width: 100%; max-height: 300px; object-fit: contain; border-radius: 10px; border: 1px solid #d1d8e0; background: #fff; }
  .patient-message.error { border-color: #f0c9c5; background: #fdf4f3; color: #8c2f25; }
  @media (max-width: 900px) {
    .profile-layout { grid-template-columns: minmax(0, 1fr); }
    .identity dl { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 20px; }
    .identity dl > div + div { margin-top: 0; }
    .ktp-view-grid { grid-template-columns: minmax(0, 1fr); }
  }
  @media (max-width: 540px) {
    .profile-card, .identity { padding: 24px; }
    .identity dl { grid-template-columns: minmax(0, 1fr); }
    .measurements, .identity-fields, .ktp-form-grid, .ktp-details { grid-template-columns: minmax(0, 1fr); }
    .profile-values { grid-template-columns: minmax(0, 1fr); gap: 22px; }
    .identity-summary dl { grid-template-columns: minmax(0, 1fr); }
  }
</style>
