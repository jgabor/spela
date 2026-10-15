<script>
  import { createEventDispatcher } from 'svelte'

  export let field
  export let fields = []
  export let semantic
  export let profileMode = 'game'

  const dispatch = createEventDispatcher()
  $: actionFields = fields.length ? fields : [field]
  // What the button does is decided by the field's source; the words for it,
  // and for the source, come from the Go vocabulary.
  $: own = profileMode === 'default' ? semantic?.origin === 'set_here' : semantic?.source === 'override'
  $: operation = own ? 'reset' : 'pin'
  $: label = semantic?.action ?? ''
  $: text = semantic ? [semantic.sourceText, semantic.ownValueSuffix].filter(Boolean).join(' · ') : ''
  // On All games, a field some games set themselves offers one action that
  // makes those games follow All games.
  $: applyOwnValues = profileMode === 'default' && semantic?.ownValueCount > 0 ? semantic.ownValueAction : ''
</script>

<span class="meta-block">
  <span class="profile-meta">
    <span>{text}</span>
    <button type="button" on:click={() => dispatch('action', { fields: actionFields, operation })}>
      {label}
    </button>
  </span>
  {#if applyOwnValues}
    <button type="button" class="apply-own-values" on:click={() => dispatch('applyOwnValue', { fields: actionFields })}>
      {applyOwnValues}
    </button>
  {/if}
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

  .meta-block {
    display: block;
  }

  .apply-own-values {
    display: block;
    margin-top: 0.15rem;
    font-size: 0.68rem;
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
