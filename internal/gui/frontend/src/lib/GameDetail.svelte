<script>
  import { createEventDispatcher, onMount } from 'svelte'
  import { GetProfile, SaveProfile, CheckDLLUpdates, UpdateDLLs, RestoreDLLs, HasDLLBackup, LaunchGame } from '../../wailsjs/go/main/App'

  export let game

  const dispatch = createEventDispatcher()

  let profile = null
  let saving = false
  let message = ''
  let dllUpdates = []
  let hasBackup = false
  let updatingDLLs = false
  let restoringDLLs = false
  let launching = false

  const srModeOptions = ['', 'off', 'ultra_performance', 'performance', 'balanced', 'quality', 'dlaa']
  const srPresetOptions = ['', 'A', 'B', 'C', 'D', 'E', 'F', 'J', 'K', 'L', 'M']
  const multiFrameOptions = [0, 1, 2, 3, 4]
  const powerMizerOptions = ['', 'adaptive', 'max']

  onMount(async () => {
    await Promise.all([loadProfile(), checkDLLUpdates()])
  })

  async function loadProfile() {
    profile = await GetProfile(game.appId)
    if (!profile) {
      profile = {
        srMode: '',
        srPreset: '',
        srOverride: false,
        fgEnabled: false,
        multiFrame: 0,
        indicator: false,
        shaderCache: false,
        threadedOptimization: false,
        powerMizer: '',
        enableHdr: false,
        enableWayland: false,
        enableNgxUpdater: false
      }
    }
  }

  async function checkDLLUpdates() {
    dllUpdates = await CheckDLLUpdates(game.appId) || []
    hasBackup = await HasDLLBackup(game.appId)
  }

  function formatError(e) {
    if (typeof e === 'string') return e
    if (e?.message) return e.message
    return String(e)
  }

  async function save() {
    saving = true
    try {
      await SaveProfile(game.appId, profile)
      message = 'Profile saved!'
      setTimeout(() => message = '', 3000)
    } catch (e) {
      message = 'Failed to save: ' + formatError(e)
    }
    saving = false
  }

  async function updateDLLs() {
    updatingDLLs = true
    try {
      await UpdateDLLs(game.appId)
      message = 'DLLs updated!'
      await checkDLLUpdates()
      setTimeout(() => message = '', 3000)
    } catch (e) {
      message = 'Failed to update: ' + formatError(e)
    }
    updatingDLLs = false
  }

  async function restoreDLLs() {
    restoringDLLs = true
    try {
      await RestoreDLLs(game.appId)
      message = 'DLLs restored!'
      await checkDLLUpdates()
      setTimeout(() => message = '', 3000)
    } catch (e) {
      message = 'Failed to restore: ' + formatError(e)
    }
    restoringDLLs = false
  }

  $: hasUpdates = dllUpdates.some(d => d.hasUpdate)

  async function launchGame() {
    launching = true
    try {
      await LaunchGame(game.appId)
      message = 'Game launched!'
      setTimeout(() => message = '', 3000)
    } catch (e) {
      message = 'Failed to launch: ' + formatError(e)
    }
    launching = false
  }
</script>

<div class="detail">
  <div class="top-bar">
    <button class="back" on:click={() => dispatch('back')}>
      ← Back
    </button>
    <button class="launch" on:click={launchGame} disabled={launching}>
      {launching ? 'Launching...' : '▶ Launch'}
    </button>
  </div>

  <h1>{game.name}</h1>

  <div class="info">
    <div class="row">
      <span class="label">App ID</span>
      <span class="value">{game.appId}</span>
    </div>
    <div class="row">
      <span class="label">Install dir</span>
      <span class="value">{game.installDir}</span>
    </div>
    {#if game.prefixPath}
      <div class="row">
        <span class="label">Prefix</span>
        <span class="value">{game.prefixPath}</span>
      </div>
    {/if}
  </div>

  {#if game.dlls && game.dlls.length > 0}
    <div class="section">
      <h2>Detected DLLs</h2>
      <div class="dll-list">
        {#each dllUpdates as dll}
          <div class="dll">
            <span class="dll-name">{dll.name}</span>
            <span class="dll-version">
              {dll.currentVersion || 'unknown'}
              {#if dll.hasUpdate}
                <span class="update-badge">→ {dll.latestVersion}</span>
              {/if}
            </span>
          </div>
        {/each}
      </div>
      <div class="dll-actions">
        {#if hasUpdates}
          <button class="update-btn" on:click={updateDLLs} disabled={updatingDLLs}>
            {updatingDLLs ? 'Updating...' : 'Update all DLLs'}
          </button>
        {/if}
        {#if hasBackup}
          <button class="restore-btn" on:click={restoreDLLs} disabled={restoringDLLs}>
            {restoringDLLs ? 'Restoring...' : 'Restore original DLLs'}
          </button>
        {/if}
      </div>
    </div>
  {/if}

  {#if profile}
    <div class="section">
      <h2>DLSS settings</h2>

      <div class="form">
        <div class="field">
          <label for="srMode">Quality mode</label>
          <select id="srMode" bind:value={profile.srMode}>
            {#each srModeOptions as opt}
              <option value={opt}>{opt || '(default)'}</option>
            {/each}
          </select>
        </div>

        <div class="field">
          <label for="srPreset">DLSS preset</label>
          <select id="srPreset" bind:value={profile.srPreset}>
            {#each srPresetOptions as opt}
              <option value={opt}>{opt || '(default)'}</option>
            {/each}
          </select>
          <span class="hint">A-F: CNN (DLSS 2/3), J-M: Transformer (DLSS 4/4.5)</span>
        </div>

        <div class="field checkbox">
          <input type="checkbox" id="srOverride" bind:checked={profile.srOverride} />
          <label for="srOverride">Override (force DLSS even if unsupported)</label>
        </div>

        <div class="field checkbox">
          <input type="checkbox" id="indicator" bind:checked={profile.indicator} />
          <label for="indicator">Show DLSS indicator</label>
        </div>

        <div class="field checkbox">
          <input type="checkbox" id="fgEnabled" bind:checked={profile.fgEnabled} />
          <label for="fgEnabled">Frame generation</label>
        </div>

        <div class="field">
          <label for="multiFrame">Multi-frame generation</label>
          <select id="multiFrame" bind:value={profile.multiFrame}>
            {#each multiFrameOptions as opt}
              <option value={opt}>{opt === 0 ? '(default)' : opt}</option>
            {/each}
          </select>
          <span class="hint">Extra frames to generate (0=off)</span>
        </div>
      </div>
    </div>

    <div class="section">
      <h2>GPU settings</h2>

      <div class="form">
        <div class="field checkbox">
          <input type="checkbox" id="shaderCache" bind:checked={profile.shaderCache} />
          <label for="shaderCache">Shader cache</label>
        </div>

        <div class="field checkbox">
          <input type="checkbox" id="threadedOptimization" bind:checked={profile.threadedOptimization} />
          <label for="threadedOptimization">Threaded optimization</label>
        </div>

        <div class="field">
          <label for="powerMizer">Power mode</label>
          <select id="powerMizer" bind:value={profile.powerMizer}>
            {#each powerMizerOptions as opt}
              <option value={opt}>{opt || '(default)'}</option>
            {/each}
          </select>
        </div>
      </div>
    </div>

    <div class="section">
      <h2>Proton settings</h2>

      <div class="form">
        <div class="field checkbox">
          <input type="checkbox" id="enableHdr" bind:checked={profile.enableHdr} />
          <label for="enableHdr">HDR</label>
        </div>

        <div class="field checkbox">
          <input type="checkbox" id="enableWayland" bind:checked={profile.enableWayland} />
          <label for="enableWayland">Wayland</label>
        </div>

        <div class="field checkbox">
          <input type="checkbox" id="enableNgxUpdater" bind:checked={profile.enableNgxUpdater} />
          <label for="enableNgxUpdater">NGX Updater</label>
        </div>
      </div>
    </div>

    <div class="actions">
      <button class="save" on:click={save} disabled={saving}>
        {saving ? 'Saving...' : 'Save profile'}
      </button>

      {#if message}
        <div class="message">{message}</div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .detail {
    max-width: 800px;
  }

  .top-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .back {
    padding: 0.5rem 1rem;
    border: none;
    border-radius: 4px;
    background-color: transparent;
    color: var(--accent-secondary);
    cursor: pointer;
    font-size: 0.9rem;
  }

  .back:hover {
    background-color: var(--bg-secondary);
  }

  .launch {
    padding: 0.5rem 1.5rem;
    border: none;
    border-radius: 4px;
    background-color: var(--success);
    color: black;
    cursor: pointer;
    font-size: 1rem;
    font-weight: 500;
  }

  .launch:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .launch:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  h1 {
    font-size: 1.5rem;
    margin-bottom: 1rem;
    color: var(--text-primary);
  }

  h2 {
    font-size: 1.1rem;
    color: var(--accent-primary);
    margin-bottom: 0.75rem;
  }

  .info {
    background-color: var(--bg-secondary);
    border-radius: 4px;
    padding: 1rem;
    margin-bottom: 1.5rem;
  }

  .row {
    display: flex;
    margin-bottom: 0.5rem;
  }

  .row:last-child {
    margin-bottom: 0;
  }

  .label {
    width: 100px;
    color: var(--text-dim);
  }

  .value {
    color: var(--text-primary);
    word-break: break-all;
  }

  .section {
    margin-bottom: 1.5rem;
  }

  .dll-list {
    background-color: var(--bg-secondary);
    border-radius: 4px;
    padding: 0.5rem;
  }

  .dll {
    display: flex;
    justify-content: space-between;
    padding: 0.5rem;
  }

  .dll-name {
    color: var(--text-primary);
  }

  .dll-version {
    color: var(--success);
  }

  .update-badge {
    color: var(--warning);
    margin-left: 0.5rem;
  }

  .dll-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.75rem;
  }

  .update-btn, .restore-btn {
    padding: 0.5rem 1rem;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.9rem;
  }

  .update-btn {
    background-color: var(--success);
    color: black;
  }

  .update-btn:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .restore-btn {
    background-color: var(--border-default);
    color: var(--text-primary);
  }

  .restore-btn:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .update-btn:disabled, .restore-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .form {
    background-color: var(--bg-secondary);
    border-radius: 4px;
    padding: 1rem;
  }

  .field {
    margin-bottom: 1rem;
  }

  .field label {
    display: block;
    color: var(--text-dim);
    margin-bottom: 0.25rem;
    font-size: 0.9rem;
  }

  .field select {
    width: 100%;
    padding: 0.5rem;
    border: 1px solid var(--border-default);
    border-radius: 4px;
    background-color: var(--bg-primary);
    color: var(--text-primary);
  }

  .field select:focus {
    outline: none;
    border-color: var(--border-focus);
  }

  .field.checkbox {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .field.checkbox label {
    margin-bottom: 0;
  }

  .field.checkbox input {
    width: 18px;
    height: 18px;
    accent-color: var(--accent-primary);
  }

  .save {
    width: 100%;
    padding: 0.75rem;
    border: none;
    border-radius: 4px;
    background-color: var(--accent-primary);
    color: var(--color-ghost-white, #F5F5FD);
    cursor: pointer;
    font-size: 1rem;
    margin-top: 0.5rem;
  }

  .save:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .save:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .message {
    margin-top: 0.75rem;
    padding: 0.5rem;
    background-color: var(--success);
    color: black;
    border-radius: 4px;
    text-align: center;
  }

  .hint {
    display: block;
    font-size: 0.8rem;
    color: var(--text-dim);
    margin-top: 0.25rem;
  }

  .actions {
    margin-top: 1.5rem;
    padding-top: 1rem;
    border-top: 1px solid var(--border-default);
  }
</style>
