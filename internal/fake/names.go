package fake

// Name pools by language region. These are ordinary, common names written for this
// project; any combination that matches a real person does so by chance.

type pool struct {
	male, female, family []string
}

var pools = map[string]pool{
	"tamil": {
		male:   []string{"Senthil", "Karthik", "Murugan", "Arun", "Vignesh", "Prakash", "Saravanan", "Dinesh", "Ganesh", "Suresh", "Rajesh", "Manikandan", "Bala", "Kumar", "Vijay", "Ashok"},
		female: []string{"Lakshmi", "Priya", "Kavitha", "Divya", "Meena", "Revathi", "Anitha", "Deepa", "Sangeetha", "Nithya", "Swetha", "Gayathri", "Bhuvana", "Yamuna", "Selvi", "Malathi"},
		family: []string{"Subramanian", "Ramasamy", "Natarajan", "Palanisamy", "Shanmugam", "Velusamy", "Chinnasamy", "Arumugam", "Krishnan", "Rajendran", "Muthusamy", "Perumal", "Thangaraj", "Annamalai", "Sundaram", "Kandasamy"},
	},
	"telugu": {
		male:   []string{"Venkatesh", "Srinivas", "Ravi", "Naresh", "Mahesh", "Kiran", "Prasad", "Ramesh", "Sai", "Chaitanya", "Harish", "Anil", "Pavan", "Naveen", "Rajasekhar", "Vamsi"},
		female: []string{"Lakshmi", "Padma", "Swathi", "Sravani", "Anusha", "Bhavani", "Sirisha", "Madhavi", "Jyothi", "Sunitha", "Kavya", "Divya", "Haritha", "Mounika", "Aruna", "Rani"},
		family: []string{"Reddy", "Rao", "Naidu", "Chowdary", "Varma", "Sharma", "Goud", "Raju", "Murthy", "Babu", "Prasad", "Sastry", "Yadav", "Gupta", "Kumar", "Setty"},
	},
	"kannada": {
		male:   []string{"Manjunath", "Basavaraj", "Raghavendra", "Shivakumar", "Girish", "Prakash", "Santosh", "Mahesh", "Naveen", "Kiran", "Suresh", "Chandrashekar", "Ravi", "Vinay", "Yogesh", "Lokesh"},
		female: []string{"Savitha", "Shobha", "Rekha", "Pushpa", "Geetha", "Roopa", "Vidya", "Asha", "Latha", "Sudha", "Chaitra", "Pallavi", "Rashmi", "Deepa", "Veena", "Nandini"},
		family: []string{"Gowda", "Hegde", "Shetty", "Patil", "Rao", "Bhat", "Kulkarni", "Naik", "Kumar", "Murthy", "Desai", "Shastri", "Acharya", "Pai", "Kamath", "Swamy"},
	},
	"malayalam": {
		male:   []string{"Anoop", "Vishnu", "Rahul", "Sreejith", "Arun", "Manoj", "Biju", "Jayan", "Sajith", "Deepak", "Nikhil", "Ajith", "Unni", "Hari", "Rajeev", "Sunil"},
		female: []string{"Anjali", "Sreelakshmi", "Divya", "Remya", "Athira", "Lekshmi", "Neethu", "Aswathy", "Sandhya", "Bindu", "Maya", "Smitha", "Parvathy", "Greeshma", "Reshma", "Veena"},
		family: []string{"Nair", "Menon", "Pillai", "Kurup", "Varghese", "Thomas", "Mathew", "George", "Panicker", "Warrier", "Namboothiri", "Joseph", "Kutty", "Unnithan", "Nambiar", "Kumar"},
	},
	"marathi": {
		male:   []string{"Sachin", "Amol", "Sandeep", "Nilesh", "Mahesh", "Ganesh", "Prashant", "Sagar", "Rohit", "Swapnil", "Vaibhav", "Ajay", "Yogesh", "Tushar", "Omkar", "Sameer"},
		female: []string{"Sneha", "Pooja", "Ashwini", "Rutuja", "Manisha", "Vaishali", "Shubhangi", "Priyanka", "Supriya", "Madhuri", "Aarti", "Sayali", "Tejaswini", "Mrunal", "Kalyani", "Smita"},
		family: []string{"Patil", "Deshmukh", "Kulkarni", "Joshi", "Pawar", "Shinde", "Jadhav", "More", "Chavan", "Gaikwad", "Kale", "Deshpande", "Bhosale", "Sawant", "Kadam", "Salunkhe"},
	},
	"gujarati": {
		male:   []string{"Jignesh", "Hardik", "Chirag", "Mehul", "Nirav", "Ketan", "Bhavesh", "Dhaval", "Parth", "Jayesh", "Kalpesh", "Mitesh", "Hitesh", "Vipul", "Tejas", "Darshan"},
		female: []string{"Hetal", "Bhumi", "Krupa", "Nisha", "Jalpa", "Foram", "Dhara", "Komal", "Pinal", "Rinkal", "Urvi", "Mittal", "Sejal", "Bhavna", "Kinjal", "Hiral"},
		family: []string{"Patel", "Shah", "Mehta", "Desai", "Joshi", "Trivedi", "Parmar", "Solanki", "Chauhan", "Dave", "Modi", "Thakkar", "Bhatt", "Pandya", "Vyas", "Gandhi"},
	},
	"punjabi": {
		male:   []string{"Gurpreet", "Harpreet", "Manpreet", "Jaspreet", "Amandeep", "Sukhwinder", "Balwinder", "Rajinder", "Kuldeep", "Navdeep", "Hardeep", "Simranjit", "Jagdeep", "Paramjit", "Lakhwinder", "Gagandeep"},
		female: []string{"Simran", "Navneet", "Harleen", "Jasleen", "Manjeet", "Kiranjit", "Amanpreet", "Gurleen", "Rupinder", "Sukhmani", "Prabhjot", "Ravneet", "Kamaljit", "Baljit", "Mandeep", "Tanvir"},
		family: []string{"Singh", "Kaur", "Sandhu", "Gill", "Sidhu", "Dhillon", "Grewal", "Brar", "Bajwa", "Randhawa", "Cheema", "Mann", "Aulakh", "Bhullar", "Sohal", "Virk"},
	},
	"bengali": {
		male:   []string{"Sourav", "Arnab", "Subhajit", "Debashish", "Partha", "Anirban", "Sandip", "Abhijit", "Rajib", "Tapas", "Suman", "Prosenjit", "Kaushik", "Arindam", "Indranil", "Biswajit"},
		female: []string{"Moumita", "Sharmistha", "Paromita", "Ananya", "Debjani", "Sudipta", "Madhumita", "Rituparna", "Tanusree", "Susmita", "Piyali", "Sanchita", "Arpita", "Mousumi", "Shreya", "Ipsita"},
		family: []string{"Banerjee", "Chatterjee", "Mukherjee", "Das", "Ghosh", "Bose", "Sen", "Dutta", "Roy", "Sarkar", "Majumdar", "Chakraborty", "Biswas", "Saha", "Mondal", "Paul"},
	},
	"hindi": {
		male:   []string{"Rahul", "Amit", "Vikas", "Sanjay", "Rakesh", "Manoj", "Deepak", "Ankit", "Rohit", "Sunil", "Pankaj", "Vivek", "Ashish", "Gaurav", "Nitin", "Alok"},
		female: []string{"Pooja", "Neha", "Priyanka", "Anjali", "Sunita", "Kavita", "Rekha", "Shweta", "Swati", "Ritu", "Nidhi", "Preeti", "Seema", "Jyoti", "Archana", "Kiran"},
		family: []string{"Sharma", "Verma", "Gupta", "Singh", "Yadav", "Mishra", "Tiwari", "Pandey", "Agarwal", "Srivastava", "Chauhan", "Dubey", "Saxena", "Tripathi", "Jain", "Rajput"},
	},
}

// statePool maps a state code to the pool most of its names would come from; the rest
// use the Hindi pool.
var statePool = map[string]string{
	"TN": "tamil", "PY": "tamil", "AP": "telugu", "TS": "telugu", "KA": "kannada", "KL": "malayalam", "LD": "malayalam",
	"MH": "marathi", "GA": "marathi", "GJ": "gujarati", "DH": "gujarati", "PB": "punjabi", "CH": "punjabi",
	"WB": "bengali", "TR": "bengali", "AS": "bengali",
}

var (
	streets = []string{"Gandhi Road", "Nehru Street", "MG Road", "Station Road", "Temple Street", "Market Road", "Bazaar Street", "College Road",
		"Lake View Road", "Park Street", "Church Road", "Main Road", "Ring Road", "Mill Road", "Hospital Road", "School Street"}
	localities = []string{"Gandhi Nagar", "Nehru Nagar", "Shanti Nagar", "Indira Nagar", "Rajaji Nagar", "Vivekananda Nagar", "Ambedkar Colony",
		"Shastri Nagar", "Lakshmi Nagar", "Sai Colony", "Teachers Colony", "Railway Colony", "Bharathi Nagar", "Azad Nagar", "Kamaraj Nagar", "Patel Nagar"}

	companyFirst = []string{"Sri Lakshmi", "Kaveri", "Annapoorna", "Ganga", "Himalaya", "Sahyadri", "Bharat", "Jai Hind", "Navjeevan", "Shree Ganesh",
		"Vinayaka", "Godavari", "Narmada", "Deccan", "Malabar", "Coromandel", "Vindhya", "Nilgiri", "Aravalli", "Konkan"}
	companyTrade = []string{"Traders", "Textiles", "Enterprises", "Agencies", "Industries", "Foods", "Electricals", "Hardware", "Pharma", "Logistics",
		"Engineering Works", "Exports", "Constructions", "Agro Products", "Steels", "Polymers", "Motors", "Stores", "Solutions", "Packaging"}
	companyForm = []string{"", "", "", " Private Limited", " Pvt Ltd", " LLP", " & Co", " and Sons"}
	emailHosts  = []string{"example.com", "example.org", "example.net", "example.in"}
)
