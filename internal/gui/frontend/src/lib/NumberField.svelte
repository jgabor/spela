<script>
  import { createEventDispatcher } from 'svelte'

  export let id
  export let value = 0
  export let unit = ''
  export let hint = ''

  const dispatch = createEventDispatcher()

  let text = String(value ?? 0)
  let shown = value
  let invalid = false

  // The field shows what is stored until the user types. When the value changes
  // from outside (a response, a failed save, another game) the text follows it,
  // even while the field still has focus.
  $: if (value !== shown) {
    shown = value
    text = String(value ?? 0)
  }

  function commit() {
    const entry = text.trim()
    if (!/^-?\d+$/.test(entry)) {
      // Not a whole number: nothing is sent and the stored value comes back.
      invalid = true
      text = String(value ?? 0)
      return
    }
    invalid = false
    const next = Number.parseInt(entry, 10)
    text = String(next)
    if (next !== value) {
      value = next
      shown = next
      dispatch('commit', next)
    }
  }
</script>

<span class="number-field">
  <input
    type="text"
    inputmode="numeric"
    {id}
    bind:value={text}
    on:change|stopPropagation={commit}
    aria-invalid={invalid}
    aria-describedby={invalid ? `${id}-error` : undefined}
  />
  {#if unit}<span class="unit">{unit}</span>{/if}
</span>
{#if invalid}
  <span class="error" id={`${id}-error`} role="alert">Enter a whole number{hint ? `. ${hint}` : ''}</span>
{:else if hint}
  <span class="number-hint">{hint}</span>
{/if}

<style>
  .number-field {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
  }

  input {
    width: 8rem;
  }

  input[aria-invalid='true'] {
    border-color: var(--error);
  }

  .unit {
    color: var(--text-dim);
    font-size: 0.8rem;
  }

  .number-hint,
  .error {
    display: block;
    margin-top: 0.2rem;
    font-size: 0.7rem;
    color: var(--text-dim);
  }

  .error {
    color: var(--error);
  }
</style>
