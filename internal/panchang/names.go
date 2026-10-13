package panchang

// Name is an element's name in English transliteration and, where the Tamil calendar
// uses its own, in Tamil.
type Name struct {
	English string `json:"name"`
	Tamil   string `json:"tamil,omitempty"`
}

var tithiNames = [15]Name{
	{"Pratipada", "பிரதமை"}, {"Dvitiya", "துவிதியை"}, {"Tritiya", "திருதியை"}, {"Chaturthi", "சதுர்த்தி"},
	{"Panchami", "பஞ்சமி"}, {"Shashthi", "சஷ்டி"}, {"Saptami", "சப்தமி"}, {"Ashtami", "அஷ்டமி"},
	{"Navami", "நவமி"}, {"Dashami", "தசமி"}, {"Ekadashi", "ஏகாதசி"}, {"Dvadashi", "துவாதசி"},
	{"Trayodashi", "திரயோதசி"}, {"Chaturdashi", "சதுர்த்தசி"}, {"Purnima", "பௌர்ணமி"},
}

var amavasya = Name{"Amavasya", "அமாவாசை"}

var nakshatraNames = [27]Name{
	{"Ashwini", "அசுவினி"}, {"Bharani", "பரணி"}, {"Krittika", "கார்த்திகை"}, {"Rohini", "ரோகிணி"},
	{"Mrigashira", "மிருகசீரிடம்"}, {"Ardra", "திருவாதிரை"}, {"Punarvasu", "புனர்பூசம்"}, {"Pushya", "பூசம்"},
	{"Ashlesha", "ஆயில்யம்"}, {"Magha", "மகம்"}, {"Purva Phalguni", "பூரம்"}, {"Uttara Phalguni", "உத்திரம்"},
	{"Hasta", "அஸ்தம்"}, {"Chitra", "சித்திரை"}, {"Swati", "சுவாதி"}, {"Vishakha", "விசாகம்"},
	{"Anuradha", "அனுஷம்"}, {"Jyeshtha", "கேட்டை"}, {"Mula", "மூலம்"}, {"Purva Ashadha", "பூராடம்"},
	{"Uttara Ashadha", "உத்திராடம்"}, {"Shravana", "திருவோணம்"}, {"Dhanishta", "அவிட்டம்"}, {"Shatabhisha", "சதயம்"},
	{"Purva Bhadrapada", "பூரட்டாதி"}, {"Uttara Bhadrapada", "உத்திரட்டாதி"}, {"Revati", "ரேவதி"},
}

var yogaNames = [27]string{
	"Vishkambha", "Priti", "Ayushman", "Saubhagya", "Shobhana", "Atiganda", "Sukarma", "Dhriti", "Shula",
	"Ganda", "Vriddhi", "Dhruva", "Vyaghata", "Harshana", "Vajra", "Siddhi", "Vyatipata", "Variyana",
	"Parigha", "Shiva", "Siddha", "Sadhya", "Shubha", "Shukla", "Brahma", "Indra", "Vaidhriti",
}

// The 60 half-tithis: Kimstughna, then the seven movable karanas eight times over, then
// the three fixed ones that close the month.
var movableKaranas = [7]string{"Bava", "Balava", "Kaulava", "Taitila", "Gara", "Vanija", "Vishti"}

func karanaName(index int) string {
	switch {
	case index == 0:
		return "Kimstughna"
	case index >= 57:
		return [3]string{"Shakuni", "Chatushpada", "Naga"}[index-57]
	}
	return movableKaranas[(index-1)%7]
}

var rashiNames = [12]Name{
	{"Mesha", "மேஷம்"}, {"Vrishabha", "ரிஷபம்"}, {"Mithuna", "மிதுனம்"}, {"Karka", "கடகம்"},
	{"Simha", "சிம்மம்"}, {"Kanya", "கன்னி"}, {"Tula", "துலாம்"}, {"Vrishchika", "விருச்சிகம்"},
	{"Dhanu", "தனுசு"}, {"Makara", "மகரம்"}, {"Kumbha", "கும்பம்"}, {"Meena", "மீனம்"},
}

// tamilMonths follow the sun's sign: Chithirai begins when it enters Mesha.
var tamilMonths = [12]Name{
	{"Chithirai", "சித்திரை"}, {"Vaikasi", "வைகாசி"}, {"Aani", "ஆனி"}, {"Aadi", "ஆடி"},
	{"Aavani", "ஆவணி"}, {"Purattasi", "புரட்டாசி"}, {"Aippasi", "ஐப்பசி"}, {"Karthigai", "கார்த்திகை"},
	{"Margazhi", "மார்கழி"}, {"Thai", "தை"}, {"Maasi", "மாசி"}, {"Panguni", "பங்குனி"},
}

// lunarMonths are named for the sign the sun is in when the month's new moon falls:
// the month that begins with the sun in Meena is Chaitra.
var lunarMonths = [12]string{
	"Chaitra", "Vaishakha", "Jyeshtha", "Ashadha", "Shravana", "Bhadrapada",
	"Ashvina", "Kartika", "Margashirsha", "Pausha", "Magha", "Phalguna",
}

var weekdayNames = [7]Name{
	{"Ravivara", "ஞாயிறு"}, {"Somavara", "திங்கள்"}, {"Mangalavara", "செவ்வாய்"}, {"Budhavara", "புதன்"},
	{"Guruvara", "வியாழன்"}, {"Shukravara", "வெள்ளி"}, {"Shanivara", "சனி"},
}

// tamilYears is the 60-year cycle; Prabhava began in April 1987.
var tamilYears = [60]Name{
	{"Prabhava", "பிரபவ"}, {"Vibhava", "விபவ"}, {"Shukla", "சுக்ல"}, {"Pramoduta", "பிரமோதூத"},
	{"Prajotpatti", "பிரசோற்பத்தி"}, {"Angirasa", "ஆங்கீரச"}, {"Srimukha", "ஸ்ரீமுக"}, {"Bhava", "பவ"},
	{"Yuva", "யுவ"}, {"Dhatu", "தாது"}, {"Ishvara", "ஈஸ்வர"}, {"Vehudhanya", "வெகுதானிய"},
	{"Pramadi", "பிரமாதி"}, {"Vikrama", "விக்கிரம"}, {"Vishu", "விஷு"}, {"Chitrabhanu", "சித்திரபானு"},
	{"Subhanu", "சுபானு"}, {"Tarana", "தாரண"}, {"Parthiva", "பார்த்திப"}, {"Viya", "விய"},
	{"Sarvajit", "சர்வசித்து"}, {"Sarvadhari", "சர்வதாரி"}, {"Virodhi", "விரோதி"}, {"Vikriti", "விகிர்தி"},
	{"Kara", "கர"}, {"Nandana", "நந்தன"}, {"Vijaya", "விஜய"}, {"Jaya", "ஜய"},
	{"Manmatha", "மன்மத"}, {"Durmukhi", "துன்முகி"}, {"Hevilambi", "ஹேவிளம்பி"}, {"Vilambi", "விளம்பி"},
	{"Vikari", "விகாரி"}, {"Sarvari", "சார்வரி"}, {"Plava", "பிலவ"}, {"Subhakrit", "சுபகிருது"},
	{"Sobhakrit", "சோபகிருது"}, {"Krodhi", "குரோதி"}, {"Vishvavasu", "விசுவாவசு"}, {"Parabhava", "பராபவ"},
	{"Plavanga", "பிலவங்க"}, {"Kilaka", "கீலக"}, {"Saumya", "சௌமிய"}, {"Sadharana", "சாதாரண"},
	{"Virodhikrit", "விரோதகிருது"}, {"Paridhavi", "பரிதாபி"}, {"Pramadicha", "பிரமாதீச"}, {"Ananda", "ஆனந்த"},
	{"Rakshasa", "ராட்சச"}, {"Nala", "நள"}, {"Pingala", "பிங்கள"}, {"Kalayukti", "காளயுக்தி"},
	{"Siddharthi", "சித்தார்த்தி"}, {"Raudri", "ரௌத்திரி"}, {"Durmati", "துன்மதி"}, {"Dundubhi", "துந்துபி"},
	{"Rudhirodgari", "ருத்ரோத்காரி"}, {"Raktakshi", "ரக்தாட்சி"}, {"Krodhana", "குரோதன"}, {"Akshaya", "அட்சய"},
}

// The eighth of the daytime each period falls in, by weekday (Sunday first).
var (
	rahuKalamPart  = [7]int{8, 2, 7, 5, 6, 4, 3}
	yamagandamPart = [7]int{5, 4, 3, 2, 1, 7, 6}
	gulikaPart     = [7]int{7, 6, 5, 4, 3, 2, 1}
)
