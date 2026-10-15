<script>
  import { onMount, createEventDispatcher, tick } from 'svelte'
  import { desktopCommands } from './desktop'
  import { emptyProfile, profileFieldDefinitions } from './profileFieldOptions.js'
  import GameDLLPane from './GameDLLPane.svelte'
  import GameProfilePane from './GameProfilePane.svelte'
  import { dllLabel } from './dllLabels.js'

  export let game
  export let profileMode = 'game'
  export let aspect = 'profile'
  export let profileSubsystem = null
  export let desktop = desktopCommands

  $: showDLLSection = profileMode === 'game' && game && aspect === 'dlls'
  $: showProfileSections = profile && aspect !== 'dlls'

  const dispatch = createEventDispatcher()

  const fieldDefinitions = new Map(profileFieldDefinitions.map(([property, field, fallback]) => [field, { property, fallback }]))

  let profile = null
  // savedProfile is the last copy the backend confirmed. A failed save returns
  // the controls to it.
  let savedProfile = null
  let message = ''
  let messageType = 'info'
  let messageTimer
  let root
  let errorMessage = ''

  // Every change is saved as it is made. Saves for one page run one after the
  // other, so their responses arrive in the order the changes were made.
  let saveQueue = Promise.resolve()
  let saveSequence = 0
  let pendingSaves = 0
  let generation = 0
  let saveMessage = ''
  let failedSave = ''
  // lastOwn remembers the value a field had before Follow, so the same button
  // puts it back.
  let lastOwn = {}

  let lastGameId = null
  let lastProfileMode = profileMode

  // What the page sets itself, from the shared words in the projection.
  $: ownSemantics = (profile?.semantics || []).filter(item =>
    profileMode === 'default' ? item.origin === 'set_here' : item.origin === 'this_game' || item.origin === 'this_game_same')
  $: ownSettingsText = profileMode === 'default' ? '' : ownSemantics.length
    ? `Own settings (${ownSemantics.length}): ${ownSemantics.map(item => `${item.label} ${item.valueText}`).join(' · ')}`
    : 'Own settings: none, every value follows All games'
  $: clearAll = profile?.clearAll ?? { action: '', count: 0, body: '' }
  $: dllFacts = game?.dlls?.length
    ? game.dlls.map(dll => `${dllLabel(dll.dllType)} ${dll.version || '?'}`).join(', ')
    : 'none detected'

  // question is the confirmation being asked, or null: its title, the profile it is
  // about, the sentence that says what happens, and what confirming runs.
  let question = null
  let cancelButton

  $: saveStatus = failedSave
    ? `Not saved: ${failedSave}`
    : pendingSaves > 0 ? 'Saving…' : 'Saved at once'

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
    generation++
    pendingSaves = 0
    saveMessage = ''
    failedSave = ''
    errorMessage = ''
    lastOwn = {}
    // The controls of the page being left must not stay on screen under the new
    // game's name: a change made there would be saved to the wrong game.
    profile = null
    savedProfile = null
    if (targetMode === 'game' && !targetAppID) {
      profile = null
      return
    }
    // Writes for the page being left finish before the next page is read.
    await saveQueue
    const loaded = targetMode === 'default'
      ? await desktop.GetDefaultProfile()
      : await desktop.GetProfile(targetAppID)
    if (profileMode !== targetMode || (targetMode === 'game' && game?.appId !== targetAppID)) return
    profile = loaded || emptyProfile()
    savedProfile = structuredClone(profile)
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

  function isCurrentTarget(mode, appID) {
    return profileMode === mode && (mode === 'default' || game?.appId === appID)
  }

  // patchesFor builds one set patch per field from the control values.
  function patchesFor(fields) {
    return fields.flatMap(field => {
      const definition = fieldDefinitions.get(field)
      if (!definition) return []
      return [{ field, operation: 'set', value: profile[definition.property] ?? definition.fallback }]
    })
  }

  function summaryFor(fields, operation, updated) {
    const semantic = (updated?.semantics || []).find(item => item.field === fields[0])
    const label = semantic?.label || fields[0]
    const suffix = profileMode === 'default' ? ' (All games)' : ''
    if (operation === 'reset') {
      return `${label} ${profileMode === 'default' ? 'uses the system default' : 'follows All games'}${suffix}`
    }
    return `${label} ${semantic?.valueText ?? ''}${suffix}`.trim()
  }

  // save sends the patches of one change at once. Responses are applied in order,
  // and only the newest one replaces the controls, so a slow older answer never
  // overwrites a later edit.
  function save(patches, fields, operation, summary = '') {
    if (!patches.length) return Promise.resolve()
    const targetMode = profileMode
    const targetAppID = game?.appId
    const myGeneration = generation
    const mySequence = ++saveSequence
    pendingSaves++
    failedSave = ''
    saveMessage = ''
    const run = async () => {
      let failure = ''
      let updated = null
      try {
        updated = targetMode === 'default'
          ? await desktop.PatchDefaultProfile(patches)
          : await desktop.PatchProfile(targetAppID, patches)
      } catch (e) {
        failure = formatError(e)
      }
      if (myGeneration !== generation || !isCurrentTarget(targetMode, targetAppID)) return
      pendingSaves = Math.max(pendingSaves - 1, 0)
      const newest = mySequence === saveSequence
      if (failure) {
        failedSave = failure
        setError('Not saved: ' + failure)
        if (newest && savedProfile) profile = structuredClone(savedProfile)
        return
      }
      savedProfile = structuredClone(updated || profile)
      if (newest && updated) profile = structuredClone(updated)
      saveMessage = 'Saved: ' + (summary || summaryFor(fields, operation, updated || profile))
      if (targetMode === 'game' && game && !game.hasProfile) await refreshGameDetails(targetAppID)
    }
    saveQueue = saveQueue.then(run)
    return saveQueue
  }

  async function handleFieldChange(event) {
    // The control has updated its bound value by the time this runs.
    await tick()
    const { fields } = event.detail
    for (const field of fields) delete lastOwn[field]
    await save(patchesFor(fields), fields, 'set')
  }

  async function handlePatchAction(event) {
    const { fields, operation } = event.detail
    if (operation === 'reset') {
      for (const field of fields) {
        const definition = fieldDefinitions.get(field)
        if (definition) lastOwn[field] = profile[definition.property] ?? definition.fallback
      }
      await save(fields.map(field => ({ field, operation: 'reset' })), fields, 'reset')
      return
    }
    // Set for this game puts back the value the field had before Follow, or
    // pins the value it follows when there is none. A profile that holds the
    // values itself (All games) cannot pin, so it sets the value shown.
    const remembered = fields.every(field => field in lastOwn)
    const valueOf = field => {
      const definition = fieldDefinitions.get(field)
      return profile[definition.property] ?? definition.fallback
    }
    const patches = remembered
      ? fields.map(field => ({ field, operation: 'set', value: lastOwn[field] }))
      : profileMode === 'default'
        ? fields.map(field => ({ field, operation: 'set', value: valueOf(field) }))
        : fields.map(field => ({ field, operation: 'pin' }))
    for (const field of fields) delete lastOwn[field]
    await save(patches, fields, 'set')
  }

  async function askConfirm(asked) {
    question = asked
    await tick()
    cancelButton?.focus()
  }

  async function openConfirm() {
    if (clearAll.count === 0) return
    await askConfirm({
      title: clearAll.action,
      name: profileMode === 'default' ? 'All games' : game?.name,
      body: clearAll.body,
      run: confirmFollowAll
    })
  }

  function closeConfirm() {
    question = null
  }

  // openApplyOwnValue asks, in words from Go, before All games is applied to the
  // games that set their own value for the field or fields.
  async function openApplyOwnValue(event) {
    const { fields } = event.detail
    let preview
    try {
      preview = await desktop.PreviewApplyToGamesWithOwnValue(fields)
    } catch (e) {
      setError(formatError(e))
      return
    }
    if (!preview?.count) return
    await askConfirm({ title: preview.action, name: 'All games', body: preview.body, run: () => applyOwnValue(fields, preview.appIds) })
  }

  // applyOwnValue makes those games follow All games. It runs in the order of
  // the saves, and the answer carries the refreshed All games page.
  function applyOwnValue(fields, appIds) {
    const myGeneration = generation
    const mySequence = ++saveSequence
    pendingSaves++
    failedSave = ''
    saveMessage = ''
    const run = async () => {
      let failure = ''
      let result = null
      try {
        result = await desktop.ApplyToGamesWithOwnValue(fields, appIds)
      } catch (e) {
        failure = formatError(e)
      }
      if (myGeneration !== generation || profileMode !== 'default') return
      pendingSaves = Math.max(pendingSaves - 1, 0)
      if (failure) {
        failedSave = failure
        setError('Not saved: ' + failure)
        return
      }
      if (result?.profile) {
        savedProfile = structuredClone(result.profile)
        if (mySequence === saveSequence) profile = structuredClone(result.profile)
      }
      if (result?.failed?.length) {
        // The header reads "Not saved: Beta (disk full)"; the banner has the whole message.
        failedSave = result.failure
        setError(result.message)
        return
      }
      saveMessage = 'Saved: ' + result.message
    }
    saveQueue = saveQueue.then(run)
    return saveQueue
  }

  // confirmFollowAll clears every setting the page holds itself in one batch.
  async function confirmFollowAll() {
    const fields = ownSemantics.map(item => item.field)
    for (const field of fields) {
      const definition = fieldDefinitions.get(field)
      if (definition) lastOwn[field] = profile[definition.property] ?? definition.fallback
    }
    const summary = profileMode === 'default'
      ? 'All settings use the system default (All games)'
      : 'All settings follow All games'
    await save(fields.map(field => ({ field, operation: 'reset' })), fields, 'reset', summary)
  }

  function handleConfirmKeydown(event) {
    if (event.key === 'Escape') {
      event.stopPropagation()
      closeConfirm()
    }
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
      <h1>All games</h1>
      <p class="default-note">Games follow these values unless they set their own.</p>
      <div class="save-line" data-state={failedSave ? 'failed' : pendingSaves > 0 ? 'saving' : 'saved'}>
        <span class="status">{saveStatus}</span>
        {#if saveMessage}<span class="saved-message">{saveMessage}</span>{/if}
      </div>
    </div>
  {:else if game}
    <div class="game-header">
      <div class="game-title">
        <h1>{game.name}</h1>
        {#if showProfileSections}
          <div class="save-line" data-state={failedSave ? 'failed' : pendingSaves > 0 ? 'saving' : 'saved'}>
            <span class="status">{saveStatus}</span>
            {#if saveMessage}<span class="saved-message">{saveMessage}</span>{/if}
          </div>
        {/if}
        <div class="info">
          <div class="row">
            <span class="label">App ID</span>
            <span class="value">{game.appId}</span>
          </div>
          <div class="row">
            <span class="label">DLLs</span>
            <span class="value">{dllFacts}</span>
          </div>
          {#if showDLLSection}
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
          {/if}
        </div>
      </div>
      {#if showDLLSection}
        <p class="launch-hint">Launch via Steam: <code>spela %command%</code></p>
      {/if}
    </div>
  {/if}

  {#if profile && showProfileSections}
    <div class="follow-all-bar">
      {#if ownSettingsText}<p class="own-settings">{ownSettingsText}</p>{/if}
      <button type="button" class="follow-all" disabled={clearAll.count === 0} on:click={openConfirm}>{clearAll.action}</button>
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
      on:patchAction={handlePatchAction}
      on:applyOwnValue={openApplyOwnValue}
      on:fieldChange={handleFieldChange}
    />
  {/if}

  {#if message}
    <div class="message" data-type={messageType}>{message}</div>
  {/if}
</div>

{#if question}
  <button type="button" class="confirm-overlay" on:click={closeConfirm} aria-label="Cancel"></button>
  <div class="confirm-panel" role="dialog" aria-modal="true" aria-label={question.title} tabindex="-1" on:keydown={handleConfirmKeydown}>
    <div class="confirm-title">{question.title}</div>
    <p class="confirm-name">{question.name}</p>
    <p class="confirm-body">{question.body}</p>
    <div class="confirm-buttons">
      <button type="button" bind:this={cancelButton} on:click={closeConfirm}>Cancel</button>
      <button type="button" class="confirm-action" on:click={() => { const run = question.run; question = null; run() }}>{question.title}</button>
    </div>
  </div>
{/if}

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

  .save-line {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem 1rem;
    font-size: 0.8rem;
    color: var(--text-dim);
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .save-line[data-state='saving'] .status {
    color: var(--warning, #ffb86c);
  }

  .save-line[data-state='failed'] .status {
    color: var(--error);
  }

  .follow-all-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem 1rem;
  }

  .own-settings {
    margin: 0;
    flex: 1;
    min-width: 240px;
    font-size: 0.8rem;
    color: var(--text-dim);
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .follow-all {
    border: 1px solid var(--border-default);
    background: var(--bg-secondary);
    color: var(--text-primary);
    padding: 0.4rem 0.8rem;
    cursor: pointer;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .follow-all:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .confirm-overlay {
    position: fixed;
    inset: 0;
    border: 0;
    background: rgba(0, 0, 0, 0.55);
    cursor: default;
  }

  .confirm-panel {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: min(34rem, calc(100vw - 2rem));
    padding: 1rem 1.25rem;
    border: 1px solid var(--border-focus, var(--border-default));
    background: var(--bg-secondary);
    color: var(--text-primary);
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .confirm-title {
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-size: 0.85rem;
    color: var(--accent-secondary);
  }

  .confirm-name,
  .confirm-body {
    margin: 0;
    font-size: 0.85rem;
  }

  .confirm-buttons {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
  }

  .confirm-buttons button {
    border: 1px solid var(--border-default);
    background: var(--bg-primary);
    color: var(--text-primary);
    padding: 0.4rem 0.8rem;
    cursor: pointer;
    font: inherit;
  }

  .confirm-buttons .confirm-action {
    border-color: var(--accent-primary);
  }

  .saved-message {
    color: var(--success);
  }

  @media (max-width: 1100px) {
    .game-header {
      flex-direction: column;
    }
  }
</style>
