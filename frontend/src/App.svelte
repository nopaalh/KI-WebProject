<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte'
  import DoctorPortal from './DoctorPortal.svelte'
  import PatientPortal from './PatientPortal.svelte'
  import { ApiError, apiLogout, type CurrentUser, type UserRole } from './api'

  type Role = UserRole
  const characters: { role: Role; label: string }[] = [
    { role: 'patient', label: 'Pasien' },
    { role: 'doctor', label: 'Dokter' },
  ]
  let selected = $state<Role | null>(null)
  let phase = $state<'choose' | 'breaking' | 'form'>('choose')
  let hovered = $state<Role | null>(null)
  let mode = $state<'login' | 'register'>('login')
  let name = $state('')
  let username = $state('')
  let password = $state('')
  let visiblePassword = $state(false)
  let pending = $state(true)
  let currentUser = $state<CurrentUser | null>(null)
  let feedback = $state('')
  let success = $state(false)
  let form = $state<HTMLFormElement>()
  let animationTimer: ReturnType<typeof setTimeout> | undefined

  onDestroy(() => clearTimeout(animationTimer))

  onMount(() => {
    const controller = new AbortController()
    void loadCurrentUser(AbortSignal.any([controller.signal, AbortSignal.timeout(15000)]))
      .catch(() => {})
      .finally(() => { pending = false })
    return () => controller.abort()
  })

  async function loadCurrentUser(signal = AbortSignal.timeout(15000)) {
    const response = await fetch('/api/users/me', {
      credentials: 'include', cache: 'no-store', signal,
      headers: { Accept: 'application/json' },
    })
    if (!response.ok) throw new Error('Unable to load current user')
    const user = await response.json()
    if (!Number.isInteger(user?.id) || user.id <= 0 ||
        typeof user.name !== 'string' || typeof user.username !== 'string' ||
        (user.role !== 'patient' && user.role !== 'doctor')) {
      throw new Error('Invalid current user response')
    }
    currentUser = user
    selected = user.role
    username = user.username
    hovered = null
    mode = 'login'
    phase = 'form'
  }

  function choose(role: Role) {
    if (phase !== 'choose' || pending) return
    selected = role
    hovered = null
    phase = 'breaking'
    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    animationTimer = setTimeout(async () => {
      if (!reducedMotion && document.startViewTransition) {
        const transition = document.startViewTransition(async () => {
          phase = 'form'
          await tick()
        })
        await transition.finished.catch(() => {})
      } else {
        phase = 'form'
      }
      await tick()
      form?.querySelector('input')?.focus({ preventScroll: true })
    }, reducedMotion ? 0 : 740)
  }

  function resetUiState(nextFeedback = '', isSuccess = false) {
    clearTimeout(animationTimer)
    currentUser = null
    mode = 'login'
    selected = null
    phase = 'choose'
    hovered = null
    password = ''
    name = ''
    username = ''
    feedback = nextFeedback
    success = isSuccess
    visiblePassword = false
  }

  function reset() {
    if (pending) return
    resetUiState()
  }

  async function logoutSession() {
    if (pending) return
    pending = true
    feedback = ''
    success = false
    try {
      await apiLogout()
      resetUiState()
    } catch (err) {
      if (err instanceof ApiError && err.isNetworkError) {
        feedback = 'Gagal menghubungi server untuk keluar. Sesi browser mungkin masih aktif.'
      } else {
        feedback = 'Gagal mengakhiri sesi di browser. Silakan coba lagi.'
      }
      throw err
    } finally {
      pending = false
    }
  }

  function handleSessionExpired() {
    resetUiState('Sesi Anda telah berakhir atau tidak valid. Silakan masuk kembali.', false)
    selected = 'patient'
    phase = 'form'
  }

  async function switchMode() {
    if (pending) return
    currentUser = null
    mode = mode === 'login' ? 'register' : 'login'
    if (mode === 'register') {
      selected = 'patient'
      phase = 'form'
      hovered = null
    }
    password = ''
    visiblePassword = false
    feedback = ''
    await tick()
    form?.querySelector('input')?.focus({ preventScroll: true })
  }

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    if (pending) return
    feedback = ''
    success = false
    if (!username.trim() || (mode === 'register' && !name.trim())) {
      feedback = 'Isi nama dan username dengan benar.'
      return
    }
    if (new TextEncoder().encode(password).length > 72) {
      feedback = 'Password maksimal 72 byte.'
      return
    }
    pending = true
    try {
      const response = await fetch(`/api/users/${mode}`, {
        method: 'POST',
        credentials: 'include',
        signal: AbortSignal.timeout(15000),
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify({
          ...(mode === 'register' ? { name: name.trim() } : {}),
          username: username.trim(),
          password,
        }),
      })
      const data = await response.json().catch(() => null)
      if (!response.ok) {
        if (response.status === 401) feedback = 'Username atau password tidak cocok.'
        else if (response.status === 409) feedback = 'Username sudah digunakan.'
        else if (response.status === 400) feedback = 'Periksa kembali data Anda.'
        else feedback = 'Layanan belum tersedia. Coba lagi nanti.'
        return
      }
      if (!data || typeof data.message !== 'string') {
        feedback = 'Respons layanan tidak sesuai. Silakan coba lagi.'
        return
      }
      password = ''
      visiblePassword = false
      if (mode === 'register') {
        mode = 'login'
        name = ''
        success = true
        feedback = 'Akun pasien berhasil dibuat. Silakan masuk.'
      } else {
        try {
          await loadCurrentUser()
        } catch {
          feedback = 'Login berhasil, tetapi profil akun belum dapat dimuat. Silakan coba masuk kembali.'
        }
      }
    } catch {
      feedback = 'Tidak dapat terhubung ke layanan. Silakan coba lagi.'
    } finally {
      pending = false
    }
  }
</script>

<svelte:head>
  <title>{currentUser ? 'Dashboard · Portal Pasien' : phase === 'choose' ? 'Portal Pasien' : mode === 'login' ? 'Masuk · Portal Pasien' : 'Daftar · Portal Pasien'}</title>
  <meta name="description" content="Pilih pasien atau dokter untuk melanjutkan ke portal." />
  <link rel="icon" type="image/svg+xml" href="/images/portal-mark.svg" />
</svelte:head>

<div class="app-shell">
  {#if currentUser?.role !== 'patient'}
  <header class="topbar">
    <button class="wordmark" type="button" onclick={() => { if (!currentUser) reset() }} disabled={pending} aria-label={currentUser ? 'Portal Pasien' : 'Portal Pasien, kembali ke pilihan peran'}>
      <img src="/images/portal-mark.svg" alt="" width="34" height="34" />
      <span>Portal Pasien<span class="wordmark-caption">Ruang kesehatan Anda</span></span>
    </button>
    <nav class="account-nav" aria-label="Navigasi akun">
      {#if currentUser}
        <span class="account-hint">{currentUser.name}</span>
        <span class="account-role">Dokter</span>
        <button class="back" type="button" onclick={() => { void logoutSession().catch(() => {}) }} disabled={pending}>{pending ? 'Memproses…' : 'Keluar'}</button>
      {:else}
        {#if phase !== 'choose'}
          <button class="back" type="button" onclick={reset} disabled={pending} aria-label="Kembali memilih pasien atau dokter">Ganti peran</button>
        {/if}
        <span class="account-hint">{mode === 'login' ? 'Belum punya akun?' : 'Sudah terdaftar?'}</span>
        <button class="account-action" type="button" onclick={switchMode} disabled={pending || phase === 'breaking'}>{mode === 'login' ? 'Buat akun' : 'Masuk ke akun'}</button>
      {/if}
    </nav>
  </header>
  {/if}

  {#if currentUser}
    {#key currentUser.id}
      {#if currentUser.role === 'patient'}
        <PatientPortal user={currentUser} onLogout={logoutSession} onUnauthorized={handleSessionExpired} />
      {:else}
        <DoctorPortal user={currentUser} onUnauthorized={handleSessionExpired} />
      {/if}
    {/key}
  {:else}
    <main class:choosing={phase !== 'form'} class:opened={phase === 'form'}>

      <header class="intro">
        <img class="app-logo" src="/images/portal-mark.svg" alt="Logo Portal Pasien" width="64" height="64" />
        <h1 class="intro-title">Selamat datang.</h1>
        <p class="intro-description">{phase === 'choose' ? 'Ruang bersama untuk pasien dan dokter.' : mode === 'login' ? 'Masuk ke akun Anda untuk melanjutkan.' : 'Buat akun pasien Anda untuk memulai.'}</p>
        {#if phase !== 'form'}<p class="role-prompt">Masuk sebagai</p>{/if}
      </header>

      <div class="scene" aria-busy={pending}>
        {#each characters as character}
          {@const isReplaced = selected !== null && selected !== character.role}
          <div class="slot {character.role}" class:replaced={isReplaced} class:chosen={selected === character.role} class:blurred={phase === 'choose' && hovered !== null && hovered !== character.role}>
            {#if !isReplaced}
              <button
                class="character"
                class:still={phase !== 'choose'}
                type="button"
                aria-label={`Pilih ${character.label}`}
                disabled={phase !== 'choose' || pending}
                onpointerenter={(event) => { if (event.pointerType !== 'touch') hovered = character.role }}
                onpointerleave={() => hovered = null}
                onfocus={() => { if (phase === 'choose') hovered = character.role }}
                onblur={() => hovered = null}
                onclick={() => choose(character.role)}
              >
                <span class="portrait" aria-hidden="true"></span>
                <span class="role-label">{character.label}</span>
              </button>
            {:else if phase === 'breaking'}
              <div class="shatter" aria-hidden="true">
                {#each Array(9) as _, index}
                  <span class="fragment" style={`--i:${index}; --x:${(index % 2 === 0 ? -1 : 1) * (22 + index * 7)}px; --r:${(index % 2 === 0 ? -1 : 1) * (3 + index)}deg; clip-path: inset(${index * 100 / 9}% 0 ${100 - (index + 1) * 100 / 9}% 0)`}></span>
                {/each}
              </div>
            {:else}
              <section class="auth" aria-labelledby="form-title">
                <div class="form-heading">
                  <h2 id="form-title">{mode === 'login' ? 'Masuk' : 'Daftar'}</h2>
                </div>
                <form bind:this={form} onsubmit={submit} aria-busy={pending}>
                  <fieldset disabled={pending}>
                    {#if mode === 'register'}
                      <div class="field">
                        <label for="name">Nama lengkap</label>
                        <input id="name" name="name" bind:value={name} autocomplete="name" placeholder="Nama Anda" required />
                      </div>
                    {/if}
                    <div class="field">
                      <label for="username">Username</label>
                      <input id="username" name="username" bind:value={username} autocomplete="username" autocapitalize="none" spellcheck="false" placeholder="Username Anda" required />
                    </div>
                    <div class="field">
                      <label for="password">Password</label>
                      <div class="password-row">
                        <input id="password" name="password" type={visiblePassword ? 'text' : 'password'} bind:value={password} autocomplete={mode === 'login' ? 'current-password' : 'new-password'} placeholder="Password Anda" required />
                        <button class="eye" type="button" aria-label={visiblePassword ? 'Sembunyikan password' : 'Tampilkan password'} aria-pressed={visiblePassword} onclick={() => visiblePassword = !visiblePassword}>
                          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" aria-hidden="true"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx="12" cy="12" r="3" />{#if visiblePassword}<path d="m3 3 18 18" />{/if}</svg>
                        </button>
                      </div>
                    </div>
                    <div class="feedback" aria-live="polite" aria-atomic="true">
                      {#if feedback}<p class:success>{feedback}</p>{/if}
                    </div>
                    <button class="submit" type="submit">{pending ? 'Memproses…' : mode === 'login' ? 'Masuk' : 'Buat akun'}</button>
                  </fieldset>
                </form>
                {#if selected === 'doctor'}
                  <p class="switch">Akun dokter disediakan oleh rumah sakit. Hak akses ditentukan oleh peran akun Anda.</p>
                {:else}
                  <p class="switch">{mode === 'login' ? 'Belum punya akun?' : 'Sudah punya akun?'} <button type="button" onclick={switchMode} disabled={pending}>{mode === 'login' ? 'Daftar' : 'Masuk'}</button></p>
                {/if}
              </section>
            {/if}
          </div>
        {/each}
      </div>
    </main>
  {/if}

</div>
