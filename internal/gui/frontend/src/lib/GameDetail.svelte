<script>
  import { onMount, createEventDispatcher, tick } from 'svelte'
  import { desktopCommands } from './desktop'
  import { emptyProfile, profileFieldDefinitions } from './profileFieldOptions.js'
  import GameDLLPane from './GameDLLPane.svelte'
  import GameProfilePane from './GameProfilePane.svelte'

  export let game
  export let profileMode = 'game'
  export let aspect = 'profile'
  export let profileSubsystem = null
  export let desktop = desktopCommands

  $: showDLLSection = profileMode === 'game' && game && aspect === 'dlls'
  $: showProfileSections = profile && aspect !== 'dlls'

  const dispatch = createEventDispatcher()

  let profile = null
  let savedProfile = null
  let pendingOperations = {}
  let saving = false
  let message = ''
  let messageType = 'info'
  let messageTimer
  let root
  let errorMessage = ''

  let lastGameId = null
  let lastProfileMode = profileMode

  onMount(async () => {
    await loadProfile()
  })

  $: if (profileMode !== lastProfileMode) {
    lastProfileMode = profileMode
    lastGameId = null
    void loadProfile()
  }

  $: if (profileMode === 'game' && game && game.appId !== lastGameId) {
    lastGameId = game.appId
    void loadProfile()
  }

  async function loadProfile() {
    const targetMode = profileMode
    const targetAppID = game?.appId
    if (targetMode === 'game' && !targetAppID) {
      profile = null
      return
    }
    const loaded = targetMode === 'default'
      ? await desktop.GetDefaultProfile()
      : await desktop.GetProfile(targetAppID)
    if (profileMode !== targetMode || (targetMode === 'game' && game?.appId !== targetAppID)) return
    profile = loaded || emptyProfile()
    savedProfile = structuredClone(profile)
    pendingOperations = {}
  }

  function formatError(e) {
    if (typeof e === 'string') return e
    if (e?.message) return e.message
    return String(e)
  }

  function clearMessageAfter(delay) {
    if (messageTimer) {
      clearTimeout(messageTimer)
    }
    messageTimer = setTimeout(() => {
      message = ''
      messageTimer = null
    }, delay)
  }

  function setMessage(nextMessage, type) {
    message = nextMessage
    messageType = type
    clearMessageAfter(3000)
  }

  function setError(text) {
    errorMessage = text
  }

  function dismissError() {
    errorMessage = ''
  }

  async function save() {
    const targetMode = profileMode
    const targetAppID = game?.appId
    const patches = profilePatches(savedProfile, profile, pendingOperations)
    saving = true
    try {
      const updated = targetMode === 'default'
        ? await desktop.PatchDefaultProfile(patches)
        : await desktop.PatchProfile(targetAppID, patches)
      const stillCurrent = isCurrentTarget(targetMode, targetAppID)
      if (stillCurrent) {
        profile = updated || profile
        savedProfile = structuredClone(profile)
        pendingOperations = {}
        if (targetMode === 'default') {
          setMessage('Default profile saved!', 'success')
        } else {
          await refreshGameDetails(targetAppID)
          if (isCurrentTarget(targetMode, targetAppID)) setMessage('Profile saved!', 'success')
        }
      }
    } catch (e) {
      if (isCurrentTarget(targetMode, targetAppID)) setError('Failed to save: ' + formatError(e))
    }
    saving = false
  }

  function isCurrentTarget(mode, appID) {
    return profileMode === mode && (mode === 'default' || game?.appId === appID)
  }

  function queuePatchAction(event) {
    const { fields, operation } = event.detail
    const next = { ...pendingOperations }
    const cancel = fields.every(field => next[field] === operation)
    for (const field of fields) {
      if (cancel) delete next[field]
      else next[field] = operation
    }
    pendingOperations = next
  }

  function clearPendingActions(event) {
    const next = { ...pendingOperations }
    for (const field of event.detail.fields) delete next[field]
    pendingOperations = next
  }

  function profilePatches(before, after, operations) {
    if (!before || !after) return []
    const patches = new Map()
    for (const [property, field, defaultValue] of profileFieldDefinitions) {
      const previous = before[property] ?? defaultValue
      const value = after[property] ?? defaultValue
      if (!Object.is(previous, value)) patches.set(field, { field, operation: 'set', value })
    }
    for (const [field, operation] of Object.entries(operations)) patches.set(field, { field, operation })
    return [...patches.values()]
  }

  async function refreshGameDetails(appID) {
    const updated = await desktop.GetGame(appID)
    if (updated && game?.appId === appID) {
      game = updated
      dispatch('gameUpdate', updated)
    }
  }

  function handleDllGameUpdate(event) {
    dispatch('gameUpdate', event.detail)
  }

  function handleDllError(event) {
    setError(event.detail)
  }

  function handleDllSuccess(event) {
    setMessage(event.detail, 'success')
  }

  export async function focusPrimary() {
    await tick()
    const focusTarget = root?.querySelector(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    )
    focusTarget?.focus()
  }
</script>

<div class="detail" bind:this={root}>
  {#if errorMessage}
    <div class="error-banner">
      <span class="error-text">{errorMessage}</span>
      <button class="error-dismiss" type="button" on:click={dismissError}>Dismiss</button>
    </div>
  {/if}
  {#if profileMode === 'default'}
    <div class="default-header">
      <h1>All games (default)</h1>
      <p class="default-note">Applies to games without their own profile.</p>
    </div>
  {:else if game}
    <div class="game-header">
      <div class="game-title">
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
      </div>
      <p class="launch-hint">Launch via Steam: <code>spela %command%</code></p>
    </div>
  {/if}

  {#if showDLLSection}
    <GameDLLPane
      bind:game
      {desktop}
      on:gameUpdate={handleDllGameUpdate}
      on:error={handleDllError}
      on:success={handleDllSuccess}
    />
  {/if}

  {#if profile && showProfileSections}
    <GameProfilePane
      bind:profile
      {profileMode}
      {profileSubsystem}
      {game}
      {desktop}
      {pendingOperations}
      on:patchAction={queuePatchAction}
      on:fieldChange={clearPendingActions}
    />

    <div class="actions">
      <button class="save" on:click={save} disabled={saving}>
        {saving ? 'Saving...' : profileMode === 'default' ? 'Save default profile' : 'Save profile'}
      </button>
    </div>
  {/if}

  {#if message}
    <div class="message" data-type={messageType}>{message}</div>
  {/if}
</div>

<style>
  .detail {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    font-family: var(--font-ui, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif);
  }

  .detail h1 {
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .game-title {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    flex: 1;
    min-width: 260px;
  }

  .game-title h1 {
    margin: 0;
  }

  .launch-hint {
    margin: 0;
    font-size: 0.8rem;
    color: var(--text-dim);
    align-self: flex-start;
  }

  .launch-hint code {
    color: var(--accent-focus);
  }

  .game-header {
    display: flex;
    justify-content: space-between;
    gap: 2rem;
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .default-header {
    padding: 1rem 1.25rem;
    border: 1px solid var(--border-default);
    border-radius: 0;
    background-color: var(--bg-secondary);
  }

  .default-header h1 {
    margin: 0 0 0.4rem;
  }

  .default-note {
    margin: 0;
    font-size: 0.85rem;
    color: var(--text-dim);
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  h1 {
    font-size: 1.6rem;
    margin-bottom: 0;
    color: var(--text-primary);
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .info {
    border: 1px solid var(--border-default);
    border-radius: 0;
    padding: 0.75rem 1rem;
    background-color: var(--bg-secondary);
  }

  .row {
    display: flex;
    margin-bottom: 0.4rem;
  }

  .row:last-child {
    margin-bottom: 0;
  }

  .label {
    width: 100px;
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-size: 0.7rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .value {
    color: var(--text-primary);
    word-break: break-all;
    font-size: 0.85rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .error-banner {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.75rem;
    padding: 0.5rem 0.75rem;
    border: 1px solid var(--error);
    background-color: var(--bg-secondary);
    color: var(--error);
    font-size: 0.8rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .error-text {
    flex: 1;
    word-break: break-word;
  }

  .error-dismiss {
    border: 1px solid var(--error);
    background: none;
    color: var(--error);
    cursor: pointer;
    padding: 0.2rem 0.6rem;
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .error-dismiss:hover {
    filter: brightness(1.2);
  }

  .save {
    width: 100%;
    padding: 0.75rem;
    border: none;
    border-radius: 0;
    background-color: var(--accent-primary);
    color: var(--color-ghost-white, #F5F5FD);
    cursor: pointer;
    font-size: 0.9rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
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
    border-radius: 0;
    text-align: center;
    background-color: var(--bg-secondary);
    border: 1px solid var(--border-default);
    text-transform: none;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .message[data-type='success'] {
    color: var(--success);
    border-color: rgba(118, 185, 0, 0.4);
  }

  .message[data-type='error'] {
    color: var(--error);
    border-color: rgba(255, 107, 107, 0.4);
  }

  .actions {
    margin-top: 0.5rem;
    padding-top: 1rem;
    border-top: 1px solid var(--border-default);
  }

  @media (max-width: 1100px) {
    .game-header {
      flex-direction: column;
    }
  }
</style>
