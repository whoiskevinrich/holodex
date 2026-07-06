import { describe, it, expect } from 'vitest';
import { countryFromNationality, countriesFromNationality } from './nationality';

describe('countryFromNationality', () => {
	it('resolves the country from a "City, Region, Country" place of birth', () => {
		expect(countryFromNationality('London, England, United Kingdom')).toEqual({
			code: 'gb',
			name: 'United Kingdom'
		});
		expect(countryFromNationality('New York City, New York, USA')).toEqual({
			code: 'us',
			name: 'United States'
		});
		expect(countryFromNationality('Paris, France')).toEqual({ code: 'fr', name: 'France' });
		expect(countryFromNationality('Tokyo, Japan')?.code).toBe('jp');
		expect(countryFromNationality('Seoul, South Korea')?.code).toBe('kr');
		expect(countryFromNationality('Rome, Lazio, Italy')?.code).toBe('it');
	});

	it('maps common synonyms and abbreviations for the country slot', () => {
		expect(countryFromNationality('Glasgow, Scotland, UK')?.code).toBe('gb');
		expect(countryFromNationality('Munich, West Germany')?.code).toBe('de');
		expect(countryFromNationality('Istanbul, Turkey')?.code).toBe('tr'); // canonical is "Türkiye"
		expect(countryFromNationality('Amsterdam, The Netherlands')?.code).toBe('nl');
	});

	it('folds diacritics when matching a country name', () => {
		expect(countryFromNationality('Ankara, Türkiye')?.code).toBe('tr');
	});

	it('resolves a plain nationality word (demonym) when there is no place of birth', () => {
		expect(countryFromNationality('British')?.code).toBe('gb');
		expect(countryFromNationality('American')?.code).toBe('us');
		expect(countryFromNationality('French')?.code).toBe('fr');
		expect(countryFromNationality('South African')?.code).toBe('za');
	});

	it('returns null for empty, blank, or unresolvable values', () => {
		expect(countryFromNationality('')).toBeNull();
		expect(countryFromNationality('   ')).toBeNull();
		expect(countryFromNationality('Atlantis')).toBeNull();
	});
});

describe('countriesFromNationality', () => {
	it('resolves each value, preserving order', () => {
		expect(countriesFromNationality(['Paris, France', 'Berlin, Germany']).map((c) => c.code)).toEqual(
			['fr', 'de']
		);
	});

	it('de-duplicates by country code', () => {
		expect(
			countriesFromNationality(['Lyon, France', 'Paris, France']).map((c) => c.code)
		).toEqual(['fr']);
	});

	it('drops unresolvable values and handles an empty list', () => {
		expect(countriesFromNationality(['Atlantis', 'Tokyo, Japan']).map((c) => c.code)).toEqual([
			'jp'
		]);
		expect(countriesFromNationality([])).toEqual([]);
	});
});
