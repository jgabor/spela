<script>
  import { createEventDispatcher } from 'svelte'

  export let field
  export let fields = []
  export let semantic
  export let profileMode = 'game'
  export let pendingOperation = ''

  const dispatch = createEventDispatcher()
  $: actionFields = fields.length ? fields : [field]
  $: operation = profileMode === 'default' || semantic?.source === 'override' ? 'reset' : 'pin'
  $: label = profileMode === 'default' ? (field === 'gpu.vrr' ? 'Reset default' : 'Unset') : operation === 'reset' ? 'Reset' : 'Pin'
  $: text = semantic
    ? profileMode === 'default'
      ? `impact ${semantic.impact} · restore ${semantic.restore}`
      : `source ${semantic.source} · impact ${semantic.impact} · restore ${semantic.restore}`
    : ''
</script>

<span class="profile-meta">
  <span>{text}</span>
  <button type="button" on:click={() => dispatch('action', { fields: actionFields, operation })}>
    {pendingOperation ? `Cancel ${label}` : label}
  </button>
</span>

<style>
  .profile-meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    margin-top: 0.2rem;
    color: var(--accent-secondary);
    font-size: 0.68rem;
    line-height: 1.3;
    text-transform: none;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  button {
    border: 0;
    padding: 0;
    background: none;
    color: var(--accent-focus);
    font: inherit;
    cursor: pointer;
  }
</style>
