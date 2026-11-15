// People and Studios' owner filters (F90 handoff): the panel's state over the list's URL
// query, its chips and the API params. A visitor gets none of it: no chips, no count, and
// nothing sent (the server 401s these params for anyone but the owner). Call during init.
import {
	entityConds,
	entityFilterCount,
	fieldChipLabel,
	loadFilterFields,
	withEntityConds,
	type EntityFilterQuery,
	type FieldCond,
	type FilterField,
	type FilterList,
	type LinkState
} from './fieldConditions';

export interface EntityFilterChip {
	id: string;
	label: string;
	negated: boolean;
	remove: () => void;
}

export class EntityFilters {
	#fields = $state<FilterField[]>([]);

	constructor(
		readonly list: FilterList,
		private get: () => EntityFilterQuery,
		private set: (q: EntityFilterQuery) => void,
		private owner: () => boolean
	) {
		$effect(() => {
			if (this.owner()) void loadFilterFields(list).then((f) => (this.#fields = f));
		});
	}

	/** The query the page acts on: empty for a visitor, whatever the URL says. */
	get query(): EntityFilterQuery {
		return this.owner() ? this.get() : {};
	}
	get link(): LinkState {
		return this.query.link ?? 'any';
	}
	get conds(): FieldCond[] {
		return entityConds(this.query);
	}
	get count(): number {
		return entityFilterCount(this.query);
	}

	setLink = (v: LinkState) => {
		const { link: _l, ...rest } = this.get();
		this.set(v === 'any' ? rest : { ...rest, link: v });
	};
	setConds = (c: readonly FieldCond[]) => this.set(withEntityConds(this.get(), c));
	clear = () => this.set({});

	/** The chips row: Linked / Not linked, then each field as "Label: has|missing". */
	get chips(): EntityFilterChip[] {
		const q = this.query;
		const out: EntityFilterChip[] = [];
		if (q.link) {
			out.push({ id: 'link', label: q.link === 'linked' ? 'Linked' : 'Not linked', negated: q.link === 'unlinked', remove: () => this.setLink('any') });
		}
		for (const c of this.conds) {
			const label = this.#fields.find((f) => f.key === c.key)?.label ?? c.key;
			out.push({
				id: `${c.has ? 'has' : 'missing'}:${c.key}`,
				label: fieldChipLabel(label, c.has),
				negated: !c.has,
				remove: () => this.setConds(this.conds.filter((x) => x.key !== c.key))
			});
		}
		return out;
	}
}
