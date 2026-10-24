import './tokens.css'

export { integrateEasy, duration, phaseToken } from './lattice'
export type { Lattice, IntegrateOptions } from './lattice'
export { LatticeClient, FendlemarkError } from './client'
export type { ClientOptions } from './client'
export { LATTICE_KEY, provideLattice, useLattice } from './keys'
export * from './types'

export { default as FmShell } from './components/FmShell.vue'
export { default as FmFacetGrid } from './components/FmFacetGrid.vue'
export { default as FmFacetPanel } from './components/FmFacetPanel.vue'
export { default as FmRunTimeline } from './components/FmRunTimeline.vue'
export { default as FmStatusBadge } from './components/FmStatusBadge.vue'
export { default as FmEntityTable } from './components/FmEntityTable.vue'
export { default as FmEmptyState } from './components/FmEmptyState.vue'
export { default as FmDeepLink } from './components/FmDeepLink.vue'

/** Package version. Kept in lockstep with the Go module. */
export const VERSION = '0.9.4'
