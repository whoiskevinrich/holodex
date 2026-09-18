package metadata

// MapExiftool exposes the exiftool JSON → Extracted parse step to external test
// packages, so an end-to-end test can feed real exiftool-shaped output through
// the scanner without spawning the binary.
var MapExiftool = mapExiftool
