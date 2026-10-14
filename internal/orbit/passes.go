package orbit

import (
	"math"
	"time"
)

// Observer is a place on the ground: geodetic latitude and longitude in degrees, height
// above the WGS 84 ellipsoid in metres.
type Observer struct {
	Latitude, Longitude, Height float64
}

// Look is where something appears from the observer: azimuth clockwise from north and
// elevation above the horizon in degrees, range in kilometres.
type Look struct {
	Azimuth, Elevation, Range float64
}

const (
	earthRadius  = 6378.137 // km, WGS 84 equatorial
	flattening   = 1 / 298.257223563
	astronomical = 149597870.7 // km
)

// position is the observer's place in the Earth-fixed frame, in km.
func (o Observer) position() Vector3 {
	lat, lon := o.Latitude*DEG2RAD, o.Longitude*DEG2RAD
	e2 := flattening * (2 - flattening)
	n := earthRadius / math.Sqrt(1-e2*math.Sin(lat)*math.Sin(lat))
	h := o.Height / 1000
	return Vector3{
		X: (n + h) * math.Cos(lat) * math.Cos(lon),
		Y: (n + h) * math.Cos(lat) * math.Sin(lon),
		Z: (n*(1-e2) + h) * math.Sin(lat),
	}
}

// fixed rotates a TEME position into the Earth-fixed frame by the sidereal time. Polar
// motion is ignored: it moves a satellite by metres.
func fixed(p Vector3, gmst float64) Vector3 {
	c, s := math.Cos(gmst), math.Sin(gmst)
	return Vector3{X: c*p.X + s*p.Y, Y: -s*p.X + c*p.Y, Z: p.Z}
}

// look turns a TEME position at a moment into azimuth, elevation and range.
func (o Observer) look(p Vector3, t time.Time) Look {
	return o.lookFixed(fixed(p, gstime(julian(t))))
}

func (o Observer) lookFixed(target Vector3) Look {
	here := o.position()
	dx, dy, dz := target.X-here.X, target.Y-here.Y, target.Z-here.Z
	lat, lon := o.Latitude*DEG2RAD, o.Longitude*DEG2RAD
	// East, north and up at the observer.
	east := -math.Sin(lon)*dx + math.Cos(lon)*dy
	north := -math.Sin(lat)*math.Cos(lon)*dx - math.Sin(lat)*math.Sin(lon)*dy + math.Cos(lat)*dz
	up := math.Cos(lat)*math.Cos(lon)*dx + math.Cos(lat)*math.Sin(lon)*dy + math.Sin(lat)*dz
	distance := math.Sqrt(dx*dx + dy*dy + dz*dz)
	azimuth := math.Atan2(east, north) * RAD2DEG
	if azimuth < 0 {
		azimuth += 360
	}
	return Look{Azimuth: azimuth, Elevation: math.Asin(up/distance) * RAD2DEG, Range: distance}
}

// sun is the Sun's position in km, in the equatorial frame of date, to about 0.01°
// (the Astronomical Almanac's low-precision formula). That is ample for telling whether
// a satellite is in sunlight and whether the sky is dark.
func sun(t time.Time) Vector3 {
	n := julian(t) - 2451545.0
	mean := math.Mod(280.460+0.9856474*n, 360) * DEG2RAD
	anomaly := math.Mod(357.528+0.9856003*n, 360) * DEG2RAD
	longitude := mean + (1.915*math.Sin(anomaly)+0.020*math.Sin(2*anomaly))*DEG2RAD
	obliquity := (23.439 - 0.0000004*n) * DEG2RAD
	distance := (1.00014 - 0.01671*math.Cos(anomaly) - 0.00014*math.Cos(2*anomaly)) * astronomical
	return Vector3{
		X: distance * math.Cos(longitude),
		Y: distance * math.Cos(obliquity) * math.Sin(longitude),
		Z: distance * math.Sin(obliquity) * math.Sin(longitude),
	}
}

// sunlit reports whether a satellite at p is outside the Earth's shadow, taken as a
// cylinder: the penumbra lasts seconds for a low satellite.
func sunlit(p, toSun Vector3) bool {
	length := math.Sqrt(toSun.X*toSun.X + toSun.Y*toSun.Y + toSun.Z*toSun.Z)
	ux, uy, uz := toSun.X/length, toSun.Y/length, toSun.Z/length
	along := p.X*ux + p.Y*uy + p.Z*uz
	if along > 0 {
		return true
	}
	cx, cy, cz := p.X-along*ux, p.Y-along*uy, p.Z-along*uz
	return math.Sqrt(cx*cx+cy*cy+cz*cz) > earthRadius
}

// Point is a moment of a pass with where to look.
type Point struct {
	Time      time.Time
	Azimuth   float64
	Elevation float64
	Range     float64
}

// Pass is one time the satellite is above the horizon. Rise is nil when it was already
// up when the search began, Set when it is still up when the search ends; a satellite
// in geostationary orbit has neither.
type Pass struct {
	Rise, Culmination, Set *Point
	// Visible is when the satellite is sunlit while the sky at the observer is dark
	// (the sun at least 6° below the horizon) and the satellite is above the horizon:
	// when it can be seen with the eye, if bright enough. Nil if never.
	VisibleFrom, VisibleUntil *time.Time
}

// PassOptions limits a search.
type PassOptions struct {
	From, Until time.Time
	// MinElevation drops passes whose highest point is lower, in degrees.
	MinElevation float64
}

// Passes finds every pass over the observer between From and Until.
func (e *Elements) Passes(o Observer, opts PassOptions) ([]Pass, error) {
	elevation := func(t time.Time) (float64, error) {
		p, _, err := e.Propagate(t)
		if err != nil {
			return 0, err
		}
		return o.look(p, t).Elevation, nil
	}
	point := func(t time.Time) (*Point, error) {
		p, _, err := e.Propagate(t)
		if err != nil {
			return nil, err
		}
		l := o.look(p, t)
		return &Point{Time: t, Azimuth: l.Azimuth, Elevation: l.Elevation, Range: l.Range}, nil
	}
	// horizon finds, to a second, when the elevation changes sign between a and b.
	horizon := func(a, b time.Time, rising bool) (time.Time, error) {
		for b.Sub(a) > time.Second {
			middle := a.Add(b.Sub(a) / 2)
			el, err := elevation(middle)
			if err != nil {
				return time.Time{}, err
			}
			if (el >= 0) == rising {
				b = middle
			} else {
				a = middle
			}
		}
		return b, nil
	}

	// Steps short enough not to miss a low satellite's shortest useful pass; slower
	// orbits are stepped more coarsely.
	step := 20 * time.Second
	if e.Period() > 225*time.Minute {
		step = 2 * time.Minute
	}
	var passes []Pass
	t := opts.From
	el, err := elevation(t)
	if err != nil {
		return nil, err
	}
	var start time.Time
	up := el >= 0
	risen := false
	if up {
		start = t
	}
	for t.Before(opts.Until) {
		next := t.Add(step)
		if next.After(opts.Until) {
			next = opts.Until
		}
		nextEl, err := elevation(next)
		if err != nil {
			return nil, err
		}
		switch {
		case !up && nextEl >= 0:
			if start, err = horizon(t, next, true); err != nil {
				return nil, err
			}
			up, risen = true, true
		case up && nextEl < 0:
			end, err := horizon(t, next, false)
			if err != nil {
				return nil, err
			}
			pass, err := e.pass(o, start, end, risen, true, point)
			if err != nil {
				return nil, err
			}
			if pass.Culmination.Elevation >= opts.MinElevation {
				passes = append(passes, pass)
			}
			up = false
		}
		t, el = next, nextEl
	}
	if up {
		pass, err := e.pass(o, start, opts.Until, risen, false, point)
		if err != nil {
			return nil, err
		}
		if pass.Culmination.Elevation >= opts.MinElevation {
			passes = append(passes, pass)
		}
	}
	return passes, nil
}

func (e *Elements) pass(o Observer, start, end time.Time, rose, set bool, point func(time.Time) (*Point, error)) (Pass, error) {
	var pass Pass
	var err error
	if rose {
		if pass.Rise, err = point(start); err != nil {
			return pass, err
		}
	}
	if set {
		if pass.Set, err = point(end); err != nil {
			return pass, err
		}
	}
	// The highest point: a golden-section search, as elevation over a pass has one peak.
	// A pass cut off by the search window may peak at its edge, so the edges compete too.
	const ratio = 0.6180339887498949
	a, b := start, end
	for b.Sub(a) > time.Second {
		span := float64(b.Sub(a))
		c := a.Add(time.Duration(span * (1 - ratio)))
		d := a.Add(time.Duration(span * ratio))
		pc, err := point(c)
		if err != nil {
			return pass, err
		}
		pd, err := point(d)
		if err != nil {
			return pass, err
		}
		if pc.Elevation >= pd.Elevation {
			b = d
		} else {
			a = c
		}
	}
	best, err := point(a.Add(b.Sub(a) / 2).Truncate(time.Second))
	if err != nil {
		return pass, err
	}
	for _, edge := range []time.Time{start, end} {
		if candidate, err := point(edge); err == nil && candidate.Elevation > best.Elevation {
			best = candidate
		}
	}
	pass.Culmination = best

	// Visibility, sampled every ten seconds across the pass.
	observer := o
	for t := start; !t.After(end); t = t.Add(10 * time.Second) {
		p, _, err := e.Propagate(t)
		if err != nil {
			return pass, err
		}
		toSun := sun(t)
		dark := observer.lookFixed(fixed(toSun, gstime(julian(t)))).Elevation < -6
		if dark && sunlit(p, toSun) && observer.look(p, t).Elevation >= 0 {
			moment := t
			if pass.VisibleFrom == nil {
				pass.VisibleFrom = &moment
			}
			pass.VisibleUntil = &moment
		}
	}
	return pass, nil
}

// Where is the point on the ground below the satellite and its height, at a moment.
type Where struct {
	Latitude, Longitude, Altitude float64 // degrees, degrees, km
	Speed                         float64 // km/s
	Sunlit                        bool
}

// SubPoint finds the point below the satellite on the WGS 84 ellipsoid.
func (e *Elements) SubPoint(t time.Time) (Where, error) {
	p, v, err := e.Propagate(t)
	if err != nil {
		return Where{}, err
	}
	f := fixed(p, gstime(julian(t)))
	longitude := math.Atan2(f.Y, f.X)
	r := math.Sqrt(f.X*f.X + f.Y*f.Y)
	e2 := flattening * (2 - flattening)
	latitude := math.Atan2(f.Z, r)
	var n float64
	for range 6 { // converges to well under a metre
		n = earthRadius / math.Sqrt(1-e2*math.Sin(latitude)*math.Sin(latitude))
		latitude = math.Atan2(f.Z+n*e2*math.Sin(latitude), r)
	}
	altitude := r/math.Cos(latitude) - n
	return Where{
		Latitude: latitude * RAD2DEG, Longitude: longitude * RAD2DEG, Altitude: altitude,
		Speed:  math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z),
		Sunlit: sunlit(p, sun(t)),
	}, nil
}
