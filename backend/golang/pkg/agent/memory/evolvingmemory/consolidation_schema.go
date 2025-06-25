package evolvingmemory

// Canonical consolidation subject buckets ─ keep the individual strings const
// so they're available both individually and as a grouped list.
const (
	ConsolidationSubjectFamilyCloseKin           = "Family & Close Kin"
	ConsolidationSubjectRomanticSexualLife       = "Romantic / Sexual Life"
	ConsolidationSubjectFriendsSocialNetwork     = "Friends & Social Network"
	ConsolidationSubjectPetsDependents           = "Pets & Dependents"
	ConsolidationSubjectEducationLearning        = "Education & Learning"
	ConsolidationSubjectCareerProfessionalLife   = "Career & Professional Life"
	ConsolidationSubjectFinancesAssets           = "Finances & Assets"
	ConsolidationSubjectGoalsProjects            = "Goals & Projects"
	ConsolidationSubjectPhysicalHealthFitness    = "Physical Health & Fitness"
	ConsolidationSubjectMentalEmotionalHealth    = "Mental & Emotional Health"
	ConsolidationSubjectHabitsDailyRoutine       = "Habits & Daily Routine"
	ConsolidationSubjectResidenceLivingSituation = "Residence & Living Situation"
	ConsolidationSubjectTravelPlaces             = "Travel & Places"
	ConsolidationSubjectHobbiesRecreation        = "Hobbies & Recreation"
	ConsolidationSubjectMediaCultureTastes       = "Media & Culture Tastes"
	ConsolidationSubjectBeliefsIdeology          = "Beliefs & Ideology"
	ConsolidationSubjectCommunityService         = "Community & Service"
	ConsolidationSubjectDigitalLifeTools         = "Digital Life & Tools"
	ConsolidationSubjectKeyLifeEventsMilestones  = "Key Life Events & Milestones"
	ConsolidationSubjectPersonalityTraits        = "Personality & Traits"
)

// ConsolidationSubjects is the aggregated, fixed-order list for comprehensive consolidation.
var ConsolidationSubjects = [...]string{
	ConsolidationSubjectFamilyCloseKin,
	ConsolidationSubjectRomanticSexualLife,
	ConsolidationSubjectFriendsSocialNetwork,
	ConsolidationSubjectPetsDependents,
	ConsolidationSubjectEducationLearning,
	ConsolidationSubjectCareerProfessionalLife,
	ConsolidationSubjectFinancesAssets,
	ConsolidationSubjectGoalsProjects,
	ConsolidationSubjectPhysicalHealthFitness,
	ConsolidationSubjectMentalEmotionalHealth,
	ConsolidationSubjectHabitsDailyRoutine,
	ConsolidationSubjectResidenceLivingSituation,
	ConsolidationSubjectTravelPlaces,
	ConsolidationSubjectHobbiesRecreation,
	ConsolidationSubjectMediaCultureTastes,
	ConsolidationSubjectBeliefsIdeology,
	ConsolidationSubjectCommunityService,
	ConsolidationSubjectDigitalLifeTools,
	ConsolidationSubjectKeyLifeEventsMilestones,
	ConsolidationSubjectPersonalityTraits,
}
