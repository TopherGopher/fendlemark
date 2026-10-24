import { inject, provide, type InjectionKey } from 'vue'
import type { Lattice } from './lattice'

/** Injection key for a Lattice created by integrateEasy. */
export const LATTICE_KEY: InjectionKey<Lattice> = Symbol('fendlemark.lattice')

/** Provide a lattice to descendant Fendlemark components. */
export function provideLattice(lattice: Lattice): void { provide(LATTICE_KEY, lattice) }

/** Inject the nearest provided lattice, or throw with a useful message. */
export function useLattice(): Lattice {
  const l = inject(LATTICE_KEY, null)
  if (!l) throw new Error('@fendlemark/ui: no lattice provided. Call provideLattice(integrateEasy({ baseUrl })) in an ancestor.')
  return l
}
