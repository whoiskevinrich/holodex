package theme

import (
	"fmt"
	"math"
	"strconv"
)

// RGB is an sRGB colour with channels in [0,1].
type RGB struct{ R, G, B float64 }

func parseHex(s string) (RGB, bool) {
	if len(s) == 0 || s[0] != '#' {
		return RGB{}, false
	}
	h := s[1:]
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return RGB{}, false
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return RGB{}, false
	}
	return RGB{float64(v>>16&0xff) / 255, float64(v>>8&0xff) / 255, float64(v&0xff) / 255}, true
}

// Hex renders "#rrggbb".
func (c RGB) Hex() string {
	q := func(v float64) int { return int(math.Round(math.Max(0, math.Min(1, v)) * 255)) }
	return fmt.Sprintf("#%02x%02x%02x", q(c.R), q(c.G), q(c.B))
}

func linear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func gamma(c float64) float64 {
	if c <= 0.0031308 {
		return 12.92 * c
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}

// oklab is the perceptual space color-mix(in oklab, …) interpolates in.
type oklab struct{ L, A, B float64 }

func toOklab(c RGB) oklab {
	r, g, b := linear(c.R), linear(c.G), linear(c.B)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	return oklab{
		0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
	}
}

func fromOklab(o oklab) RGB {
	l := o.L + 0.3963377774*o.A + 0.2158037573*o.B
	m := o.L - 0.1055613458*o.A - 0.0638541728*o.B
	s := o.L - 0.0894841775*o.A - 1.2914855480*o.B
	l, m, s = l*l*l, m*m*m, s*s*s
	clamp := func(v float64) float64 { return math.Max(0, math.Min(1, v)) }
	return RGB{
		gamma(clamp(4.0767416621*l - 3.3077115913*m + 0.2309699292*s)),
		gamma(clamp(-1.2684380046*l + 2.6097574011*m - 0.3413193965*s)),
		gamma(clamp(-0.0041960863*l - 0.7034186147*m + 1.7076147010*s)),
	}
}

// Mix is color-mix(in oklab, a, b pctB%).
func Mix(a, b RGB, pctB float64) RGB {
	t := pctB / 100
	oa, ob := toOklab(a), toOklab(b)
	return fromOklab(oklab{oa.L*(1-t) + ob.L*t, oa.A*(1-t) + ob.A*t, oa.B*(1-t) + ob.B*t})
}

// relLum is WCAG 2 relative luminance.
func relLum(c RGB) float64 {
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}

// ContrastRatio is WCAG 2 contrast, >= 1, independent of argument order.
func ContrastRatio(a, b RGB) float64 {
	la, lb := relLum(a), relLum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// toCielab is CIE L*a*b* (D65), used only for the ΔE*ab gate.
func toCielab(c RGB) (float64, float64, float64) {
	r, g, b := linear(c.R), linear(c.G), linear(c.B)
	x := (0.4124564*r + 0.3575761*g + 0.1804375*b) / 0.95047
	y := 0.2126729*r + 0.7151522*g + 0.0721750*b
	z := (0.0193339*r + 0.1191920*g + 0.9503041*b) / 1.08883
	f := func(t float64) float64 {
		if t > 0.008856 {
			return math.Cbrt(t)
		}
		return 7.787*t + 16.0/116
	}
	fx, fy, fz := f(x), f(y), f(z)
	return 116*fy - 16, 500 * (fx - fy), 200 * (fy - fz)
}

// DeltaE is CIE76 ΔE*ab between two colours.
func DeltaE(a, b RGB) float64 {
	l1, a1, b1 := toCielab(a)
	l2, a2, b2 := toCielab(b)
	return math.Sqrt((l1-l2)*(l1-l2) + (a1-a2)*(a1-a2) + (b1-b2)*(b1-b2))
}
