### Overview
Your job is to build a simple real-time fan engagment Analytics Service that demonstrates your hands-on technical capabilities in the context of Fanfinity's platform needs. This microservice will process live match events and generate engagement metrics, core functionality for our fan engagement platform.

### The Challenge
Business Context
During a major match (e.g., Al-Hilal vs. Al-Nassr), Fanfinity expects:
- 50,000+ concurrent users
- Event spikes of 1,000+ requests/second during goals
- Sub-200ms API response times
- Zero data loss during traffic surges
Your service should demonstrate architectural decisions that address these requirements.
What You'll Build
Create a standalone microservice that:
1.​ Ingests match events via REST API (goals, cards, substitutions, etc.)
2.​ Processes events in real-time and calculates engagement metrics
3.​ Exposes metrics via API for consumption by frontend applications
4.​ Demonstrates scalability patterns suitable for handling match-day traffic spikes
Technical Requirements
Core FunctionalityHTML
POST /api/events
- Accept match events (goal, card, substitution, match_start,
match_end)
- Validate event data
- Process asynchronously if needed
- Return 202 Accepted with event ID
GET /api/matches/{matchId}/metrics
- Return real-time engagement metrics:
- Total events
- Events by type
- Peak engagement periods (events per minute)
- Response time percentiles (p50, p95, p99)
GET /metrics
- Prometheus-compatible metrics
- Request rates, error rates, latency
- Business metrics (events processed, matches active)
Data Model (Suggested)
None
Match Event:
- eventId (UUID)
- matchId (string)- eventType (enum: goal, yellow_card, red_card, substitution,
etc.)
- timestamp (ISO 8601)
- teamId (string)
- playerId (optional string)
- metadata (JSON object for extensibility)
Technology Stack (Your Choice)
Language Options: Choose one that best demonstrates your expertise
●​
●​
●​
●​
●​
Go (preferred for high-performance services)
Python (FastAPI/Django)
Node.js (TypeScript/Express)
Java/Kotlin (Spring Boot)
Rust (if you want to impress)
Required Components
●​
●​
●​
●​
●​
REST API framework
Data persistence (PostgreSQL, MongoDB, or Redis)
Containerization (Docker)
CI/CD pipeline (GitHub Actions)
Infrastructure as Code (optional but valued)
Deliverables
1. Source Code Repository
Must include
●​ Complete, working application code
●​ README.md with:
○​ Architecture overview and key decisions
○​ Setup instructions
○​ API documentation
○​ Assumptions and trade-offs made○​ Known limitations and production considerations
●​ Dockerfile with multi-stage build
●​ docker-compose.yml for local development
●​ .env.example with configuration template
2. CI/CD Pipeline
GitHub Actions workflow that:
●​
●​
●​
●​
●​
Runs on push to main and pull requests
Executes linting and code quality checks
Runs unit tests with coverage reporting (>70% target)
Builds Docker image
Tags images with commit SHA and version
4. Testing & Quality
Demonstrate
●​ Unit tests for core business logic
5. Documentation
Include brief written responses (1-2 paragraphs each)
1.​ Architecture Decisions: Why did you choose this language, framework, and data
store?
2.​ Scalability Approach: How does your design handle 10x traffic? What would break
first?
3.​ Production Readiness: What's missing for production? What would you add with more
time?
Evaluation Criteria
Code Quality (25%)
●​
●​
●​
●​
Clean, maintainable code following language conventions
Proper error handling and logging
Configuration management (12-factor app principles)
Code organization and structure
DevOps Practices (25%)
●​ Effective CI/CD pipeline●​ Container optimization (image size, layers, security)
●​ Infrastructure as Code quality
●​ Deployment strategy
System Design (25%)
●​
●​
●​
●​
API design and REST principles
Performance considerations
Scalability patterns
Observability (logging, metrics, tracing)
Testing & Reliability (15%)
●​
●​
●​
●​
Test coverage and quality
Error handling and resilience
Health checks and graceful degradation
Load testing approach
Documentation (10%)
●​
●​
●​
●​
Clear setup instructions
Architectural reasoning
Trade-off explanations
Production considerations
Constraints
●​ Time Commitment: Maximum 3 hours (we're timing demonstration of skills, not
perfection)
●​ Scope: Focus on one microservice only
●​ Complexity: Don't over-engineer, show you can balance pragmatism with quality
●​ External Services: Mock all external dependencies
Submission
1.​ Create a public GitHub repository with your solution
2.​ Create a brief video (3-5 minutes, Loom/unlisted YouTube) walking through:
○​ Quick code overview
○​ Running the application locally
○​ Triggering the CI/CD pipeline
○​ Demonstrating key API endpoints
○​ Discussing one interesting technical decision
3.​ Submit repository URL and video linkBonus Points (Optional)
These are NOT required but demonstrate advanced capabilities:
●​ Comprehensive API documentation (OpenAPI/Swagger)
What We're Looking For
This task reveals
●​
●​
●​
●​
●​
Can you ship? Working code that actually runs
Production mindset: Do you think beyond "works on my machine"?
DevOps fluency: Are you comfortable with the full stack?
Pragmatic engineering: Can you balance speed with quality?
Communication: Can you explain your decisions clearly?
Remember: We'd rather see a simple solution executed excellently than a complex solution
executed poorly. Show us you can build the foundation Fanfinity needs.
Deliverables
Submit: Candidates may submit their work in any format they feel best showcases their
thinking, technical depth, problem solving, and communication style. This can be a written
document, a presentation deck, a technical architecture outline, or a mix of formats.
Present: Candidates will present their work in a dedicated session with up to 45 minutes for the
presentation followed by 30 minutes of discussion with the CEO and Studio leadership.
Timeline: Submit 72 hours before interview; be prepared to defend trade-offs and adapt your
plan based on constraints we'll introduce during discussion
This case study assesses your ability to think strategically, execute pragmatically, and lead
effectively at the intersection of technology and business. Show us why you're the right CTO to
build Fanfinity's technology foundation.
