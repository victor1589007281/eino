// Package agent provides the core agent implementations for Interview Expert.
package agent

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// InterviewPhase represents the phase of the interview process.
type InterviewPhase int

const (
	// PhaseJDAnalysis represents JD analysis phase.
	PhaseJDAnalysis InterviewPhase = iota
	// PhaseQuestionDesign represents question design phase.
	PhaseQuestionDesign
	// PhaseInterview represents active interview phase.
	PhaseInterview
	// PhaseEvaluation represents evaluation phase.
	PhaseEvaluation
	// PhaseReport represents report generation phase.
	PhaseReport
)

// String returns the string representation of InterviewPhase.
func (p InterviewPhase) String() string {
	switch p {
	case PhaseJDAnalysis:
		return "jd_analysis"
	case PhaseQuestionDesign:
		return "question_design"
	case PhaseInterview:
		return "interview"
	case PhaseEvaluation:
		return "evaluation"
	case PhaseReport:
		return "report"
	default:
		return "unknown"
	}
}

// QuestionDifficulty represents the difficulty level of a question.
type QuestionDifficulty int

const (
	DifficultyBasic QuestionDifficulty = iota
	DifficultyIntermediate
	DifficultyAdvanced
	DifficultyExpert
)

// String returns the string representation of QuestionDifficulty.
func (d QuestionDifficulty) String() string {
	switch d {
	case DifficultyBasic:
		return "basic"
	case DifficultyIntermediate:
		return "intermediate"
	case DifficultyAdvanced:
		return "advanced"
	case DifficultyExpert:
		return "expert"
	default:
		return "unknown"
	}
}

// QuestionType represents the type of interview question.
type QuestionType int

const (
	QuestionTypeTechnical QuestionType = iota
	QuestionTypeBehavioral
	QuestionTypeScenario
	QuestionTypeCoding
	QuestionTypeSystemDesign
	QuestionTypeProjectExperience
	QuestionTypeSoftSkills
)

// String returns the string representation of QuestionType.
func (t QuestionType) String() string {
	switch t {
	case QuestionTypeTechnical:
		return "technical"
	case QuestionTypeBehavioral:
		return "behavioral"
	case QuestionTypeScenario:
		return "scenario"
	case QuestionTypeCoding:
		return "coding"
	case QuestionTypeSystemDesign:
		return "system_design"
	case QuestionTypeProjectExperience:
		return "project_experience"
	case QuestionTypeSoftSkills:
		return "soft_skills"
	default:
		return "unknown"
	}
}

// SubAgent defines the interface for interview sub-agents.
type SubAgent interface {
	adk.Agent

	// CanHandle returns true if the agent can handle the given phase.
	CanHandle(ctx context.Context, phase InterviewPhase) bool

	// GetCapabilities returns the capabilities of the agent.
	GetCapabilities() []string
}

// JobDescription represents a parsed job description.
type JobDescription struct {
	ID                string              `json:"id"`
	Title             string              `json:"title"`
	Company           string              `json:"company,omitempty"`
	Department        string              `json:"department,omitempty"`
	Level             string              `json:"level"` // junior, mid, senior, lead, principal
	RequiredSkills    []Skill             `json:"required_skills"`
	PreferredSkills   []Skill             `json:"preferred_skills,omitempty"`
	Responsibilities  []string            `json:"responsibilities"`
	Requirements      []string            `json:"requirements"`
	YearsExperience   ExperienceRange     `json:"years_experience"`
	EducationRequired string              `json:"education_required,omitempty"`
	Keywords          []string            `json:"keywords,omitempty"`
	HiringIntent      *HiringIntent       `json:"hiring_intent,omitempty"`
	RawContent        string              `json:"raw_content"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

// Skill represents a skill with proficiency level.
type Skill struct {
	Name           string  `json:"name"`
	Category       string  `json:"category"` // programming, framework, database, cloud, soft_skill, etc.
	Priority       int     `json:"priority"` // 1=must-have, 2=important, 3=nice-to-have
	ExpectedLevel  string  `json:"expected_level"` // beginner, intermediate, advanced, expert
	Weight         float64 `json:"weight"` // Weight for scoring
	Keywords       []string `json:"keywords,omitempty"`
}

// ExperienceRange represents a range of years of experience.
type ExperienceRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// HiringIntent represents the hiring intent analysis result.
type HiringIntent struct {
	PrimaryFocus       string   `json:"primary_focus"` // technical, leadership, specialist, generalist
	TeamRole           string   `json:"team_role"` // individual_contributor, tech_lead, manager
	GrowthPotential    string   `json:"growth_potential"` // high, medium, low
	Urgency            string   `json:"urgency"` // urgent, normal, flexible
	KeyCompetencies    []string `json:"key_competencies"`
	CultureFit         []string `json:"culture_fit_traits"`
	PotentialChallenges []string `json:"potential_challenges,omitempty"`
}

// InterviewPlan represents the interview plan.
type InterviewPlan struct {
	ID              string            `json:"id"`
	JobDescriptionID string           `json:"job_description_id"`
	TotalDuration   int               `json:"total_duration_minutes"`
	Rounds          []InterviewRound  `json:"rounds"`
	FocusAreas      []FocusArea       `json:"focus_areas"`
	DifficultyLevel QuestionDifficulty `json:"difficulty_level"`
	Created         string            `json:"created"`
	Metadata        map[string]any    `json:"metadata,omitempty"`
}

// InterviewRound represents a round in the interview.
type InterviewRound struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Type          string           `json:"type"` // screening, technical, behavioral, system_design, etc.
	Duration      int              `json:"duration_minutes"`
	Questions     []Question       `json:"questions"`
	FocusSkills   []string         `json:"focus_skills"`
	PassCriteria  string           `json:"pass_criteria"`
}

// FocusArea represents a focus area in the interview.
type FocusArea struct {
	Name        string   `json:"name"`
	Weight      float64  `json:"weight"`
	Skills      []string `json:"skills"`
	MinScore    float64  `json:"min_score"`
}

// Question represents an interview question.
type Question struct {
	ID              string             `json:"id"`
	Content         string             `json:"content"`
	Type            QuestionType       `json:"type"`
	Difficulty      QuestionDifficulty `json:"difficulty"`
	Category        string             `json:"category"`
	Skills          []string           `json:"skills"`
	ExpectedAnswer  string             `json:"expected_answer,omitempty"`
	FollowUps       []string           `json:"follow_ups,omitempty"`
	EvaluationCriteria []EvaluationCriterion `json:"evaluation_criteria"`
	TimeLimit       int                `json:"time_limit_minutes,omitempty"`
	Weight          float64            `json:"weight"`
	Hints           []string           `json:"hints,omitempty"`
}

// EvaluationCriterion represents a criterion for evaluating answers.
type EvaluationCriterion struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	MaxScore    float64 `json:"max_score"`
	Weight      float64 `json:"weight"`
}

// Candidate represents a candidate being interviewed.
type Candidate struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Email       string         `json:"email,omitempty"`
	Resume      *Resume        `json:"resume,omitempty"`
	SessionID   string         `json:"session_id"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Resume represents a candidate's resume.
type Resume struct {
	Education       []Education      `json:"education,omitempty"`
	Experience      []WorkExperience `json:"experience,omitempty"`
	Skills          []string         `json:"skills,omitempty"`
	Projects        []Project        `json:"projects,omitempty"`
	Certifications  []string         `json:"certifications,omitempty"`
	TotalYearsExp   int              `json:"total_years_experience"`
	Summary         string           `json:"summary,omitempty"`
}

// Education represents educational background.
type Education struct {
	Degree     string `json:"degree"`
	Field      string `json:"field"`
	School     string `json:"school"`
	Year       int    `json:"year"`
}

// WorkExperience represents work experience.
type WorkExperience struct {
	Company     string   `json:"company"`
	Title       string   `json:"title"`
	Duration    string   `json:"duration"`
	Description string   `json:"description"`
	Skills      []string `json:"skills,omitempty"`
}

// Project represents a project.
type Project struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Role        string   `json:"role"`
	Skills      []string `json:"skills"`
	Impact      string   `json:"impact,omitempty"`
}

// InterviewSession represents an active interview session.
type InterviewSession struct {
	ID              string             `json:"id"`
	Candidate       *Candidate         `json:"candidate"`
	Plan            *InterviewPlan     `json:"plan"`
	CurrentRound    int                `json:"current_round"`
	CurrentQuestion int                `json:"current_question"`
	Answers         []Answer           `json:"answers"`
	StartTime       string             `json:"start_time"`
	Status          string             `json:"status"` // in_progress, completed, paused, cancelled
	Metadata        map[string]any     `json:"metadata,omitempty"`
}

// Answer represents a candidate's answer to a question.
type Answer struct {
	QuestionID    string          `json:"question_id"`
	Content       string          `json:"content"`
	Duration      int             `json:"duration_seconds"`
	Evaluation    *AnswerEval     `json:"evaluation,omitempty"`
	FollowUpAsked []string        `json:"follow_up_asked,omitempty"`
	Timestamp     string          `json:"timestamp"`
}

// AnswerEval represents the evaluation of an answer.
type AnswerEval struct {
	Score           float64                  `json:"score"`
	MaxScore        float64                  `json:"max_score"`
	Breakdown       map[string]float64       `json:"breakdown"` // criterion -> score
	Strengths       []string                 `json:"strengths"`
	Weaknesses      []string                 `json:"weaknesses"`
	Notes           string                   `json:"notes,omitempty"`
	SkillDemonstrated []SkillDemonstration   `json:"skill_demonstrated,omitempty"`
}

// SkillDemonstration represents demonstration of a skill.
type SkillDemonstration struct {
	Skill    string  `json:"skill"`
	Level    string  `json:"level"` // none, basic, intermediate, advanced, expert
	Evidence string  `json:"evidence"`
	Score    float64 `json:"score"`
}

// SkillProfile represents the candidate's overall skill profile.
type SkillProfile struct {
	CandidateID   string                 `json:"candidate_id"`
	Skills        map[string]SkillScore  `json:"skills"`
	Strengths     []string               `json:"strengths"`
	Weaknesses    []string               `json:"weaknesses"`
	Recommendations []string             `json:"recommendations"`
	OverallLevel  string                 `json:"overall_level"`
}

// SkillScore represents the score for a specific skill.
type SkillScore struct {
	Name          string  `json:"name"`
	Score         float64 `json:"score"`
	MaxScore      float64 `json:"max_score"`
	Level         string  `json:"level"`
	Confidence    float64 `json:"confidence"`
	QuestionCount int     `json:"question_count"`
	Evidence      []string `json:"evidence,omitempty"`
}

// MatchResult represents the job matching result.
type MatchResult struct {
	CandidateID      string              `json:"candidate_id"`
	JobDescriptionID string              `json:"job_description_id"`
	OverallMatch     float64             `json:"overall_match"` // 0-100
	SkillMatch       float64             `json:"skill_match"`
	ExperienceMatch  float64             `json:"experience_match"`
	CultureMatch     float64             `json:"culture_match,omitempty"`
	SkillGaps        []SkillGap          `json:"skill_gaps"`
	SkillSurplus     []string            `json:"skill_surplus,omitempty"`
	Recommendation   string              `json:"recommendation"` // strong_hire, hire, maybe, no_hire
	Confidence       float64             `json:"confidence"`
	Notes            string              `json:"notes,omitempty"`
}

// SkillGap represents a gap in skills.
type SkillGap struct {
	Skill          string  `json:"skill"`
	RequiredLevel  string  `json:"required_level"`
	ActualLevel    string  `json:"actual_level"`
	Importance     string  `json:"importance"` // critical, important, nice_to_have
	Recommendation string  `json:"recommendation,omitempty"`
}

// InterviewReport represents the final interview report.
type InterviewReport struct {
	ID               string            `json:"id"`
	SessionID        string            `json:"session_id"`
	Candidate        *Candidate        `json:"candidate"`
	JobDescription   *JobDescription   `json:"job_description"`
	ExecutiveSummary string            `json:"executive_summary"`
	SkillProfile     *SkillProfile     `json:"skill_profile"`
	MatchResult      *MatchResult      `json:"match_result"`
	RoundSummaries   []RoundSummary    `json:"round_summaries"`
	OverallScore     float64           `json:"overall_score"`
	Recommendation   string            `json:"recommendation"`
	Strengths        []string          `json:"strengths"`
	AreasForImprovement []string       `json:"areas_for_improvement"`
	InterviewerNotes string            `json:"interviewer_notes,omitempty"`
	NextSteps        []string          `json:"next_steps,omitempty"`
	Diagrams         []Diagram         `json:"diagrams,omitempty"`
	GeneratedAt      string            `json:"generated_at"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
}

// RoundSummary represents a summary of an interview round.
type RoundSummary struct {
	RoundID       string   `json:"round_id"`
	RoundName     string   `json:"round_name"`
	Score         float64  `json:"score"`
	MaxScore      float64  `json:"max_score"`
	Passed        bool     `json:"passed"`
	Highlights    []string `json:"highlights"`
	Concerns      []string `json:"concerns,omitempty"`
	Notes         string   `json:"notes,omitempty"`
}

// Diagram represents a diagram in the report.
type Diagram struct {
	Type    DiagramType `json:"type"`
	Title   string      `json:"title"`
	Content string      `json:"content"` // Mermaid code
}

// DiagramType represents the type of diagram.
type DiagramType string

const (
	DiagramTypeSkillRadar    DiagramType = "skill_radar"
	DiagramTypeScoreBar      DiagramType = "score_bar"
	DiagramTypeComparison    DiagramType = "comparison"
	DiagramTypeFlow          DiagramType = "flow"
	DiagramTypeTimeline      DiagramType = "timeline"
)

// AgentMessage represents a message in agent communication.
type AgentMessage struct {
	Role    string             `json:"role"`
	Content string             `json:"content"`
	Tools   []*schema.ToolInfo `json:"tools,omitempty"`
	Extra   map[string]any     `json:"extra,omitempty"`
}

// AgentResponse represents a response from an agent.
type AgentResponse struct {
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}
