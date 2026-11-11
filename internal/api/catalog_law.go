package api

// The new criminal laws and the old ones (law.go).

func withLaw(groups []apiGroup) []apiGroup {
	return append(groups, apiGroup{
		ID: "law", Category: "language", Title: "IPC to BNS section finder",
		Tagline:   "Which section of the Bharatiya Nyaya Sanhita replaced IPC 302, and the same for CrPC to BNSS and the Evidence Act to BSA.",
		Intro:     "India's new criminal laws came into force on 1 July 2024: the Bharatiya Nyaya Sanhita (BNS) replaced the Indian Penal Code (IPC), the Bharatiya Nagarik Suraksha Sanhita (BNSS) the Code of Criminal Procedure (CrPC), and the Bharatiya Sakshya Adhiniyam (BSA) the Indian Evidence Act (IEA). Look a section up either way, or search the titles. From the Bureau of Police Research and Development's correspondence tables; the section titles are the Acts' own.",
		SourceKey: "law",
		Endpoints: []endpoint{
			{Path: "/v1/law/{law}/{section}", Summary: "A section and its counterpart", Example: "/v1/law/ipc/302",
				Doc: "With a new law (bns, bnss, bsa), the section (all its subsections for 103; one for 103(1)) and the old section it stands for. With an old law (ipc, crpc, iea), every new section the old one went to. new_provision marks a section with no counterpart.",
				Params: []param{{Name: "law", In: "path", Type: "string", Required: true, Enum: []string{"bns", "bnss", "bsa", "ipc", "crpc", "iea"}, Doc: "Which law the section number belongs to."},
					{Name: "section", In: "path", Type: "string", Required: true, Doc: "Such as 302, 498A, 103(1)."}}},
			{Path: "/v1/law/{law}", Summary: "Search section titles", Example: "/v1/law/bns?q=murder",
				Doc: "The new law's sections whose titles contain the words (an old law's name searches its replacement), at most 50.",
				Params: []param{{Name: "law", In: "path", Type: "string", Required: true, Enum: []string{"bns", "bnss", "bsa", "ipc", "crpc", "iea"}, Doc: "Which law."},
					{Name: "q", In: "query", Type: "string", Required: true, Doc: "3 to 100 characters, such as cheating or bail."}}},
		},
	})
}
