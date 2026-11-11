package api

// Hospitals (hospitals.go).

func withHospitals(groups []apiGroup) []apiGroup {
	return append(groups, apiGroup{
		ID: "hospitals", Category: "places", Title: "Hospitals",
		Tagline:   "About 30,000 hospitals across India with their kind, system of medicine, phone and position, from the National Hospital Directory.",
		Intro:     "The Ministry of Health's National Hospital Directory: government and private hospitals, nursing homes and clinics, allopathic and AYUSH, with address, phone numbers, emergency and ambulance numbers where given, specialties, and a position for about a third. As of May 2016: check before you rely on one. In an emergency dial 112 or 108.",
		SourceKey: "hospitals",
		Endpoints: []endpoint{
			{Path: "/v1/hospitals", Summary: "Hospitals near a point, by PIN code, district or name", Live: true, Example: "/v1/hospitals?pincode=637001",
				Doc: "One of near, pincode, district or q. near gives the ten nearest within about 50 km, with their distance; the others up to 100, by name.",
				Params: []param{{Name: "near", In: "query", Type: "string", Doc: "latitude,longitude."},
					{Name: "pincode", In: "query", Type: "string", Doc: "Six digits."},
					{Name: "district", In: "query", Type: "string", Doc: "A district, such as Namakkal; add state to narrow it."},
					{Name: "state", In: "query", Type: "string", Doc: "With district or q."},
					{Name: "q", In: "query", Type: "string", Doc: "Part of a hospital's name, 3 to 80 characters."},
					{Name: "system", In: "query", Type: "string", Doc: "Only this system of medicine: Allopathic, Ayurveda, Homeopathy, Siddha, Unani."}}},
		},
	})
}
