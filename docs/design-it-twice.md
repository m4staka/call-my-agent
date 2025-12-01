# Design It Twice
## Core Principle: Always Design It Twice
- Principle from Philosophy of Software Design by John Osterhout -
Before implementing any significant feature, module, or API, **always create at least two different design approaches**. The second design is almost always superior to the first.
### When to Apply Design It Twice
- New modules or classes
- Public APIs or interfaces
- Complex algorithms or data structures
- Error handling strategies
- System architecture decisions
- Database schemas or data models
## Managing Complexity Through Decomposition
### Deep Modules Principle
- **Create deep modules**: Modules should have simple interfaces that hide significant internal complexity
- **Avoid shallow modules**: Don't create modules that barely hide any complexity
- **Interface design**: The best modules are those where the interface is much simpler than the implementation
### Modular Design Guidelines
1. **Hide complexity** behind clean interfaces rather than exposing it
2. **Eliminate special cases** whenever possible - they add unnecessary complexity
3. **Break large systems** into independent, manageable units
4. **Design from the caller's perspective** - what would be most convenient for the user of this code?
## Implementation Guidelines
### Before Writing Code
1. **Sketch two different approaches** for the solution
2. **Compare trade-offs** between the approaches
3. **Consider the interface** from the caller's perspective
4. **Identify potential complexity** and how to hide or eliminate it
### Error Handling Strategy
- **Define errors out of existence** when possible through good design
- **Don't ignore necessary error checks** - but design systems to prevent errors from occurring
- **Make error states obvious** and easy to handle for callers
## Browser Extension Specific Applications
### Component Design
- Design React components twice: first pass for functionality, second pass for API elegance
- Consider both the props interface and the internal implementation
- Hide complex state management behind simple component APIs
### API Integrations
- Design service layer APIs twice: consider both ease of use and maintainability
- Abstract away browser-specific APIs behind clean interfaces
- Make async operations easy and safe to use
### State Management
- Design state structures twice: first for features needed, second for developer experience
- Hide Redux/state complexity behind simple action creators and selectors
- Consider both performance and debugging when designing state shape
## Success Metrics
A good design following "design it twice":
- **The interface is much simpler** than the implementation
- **Callers rarely need documentation** beyond type signatures
- **Adding new features** doesn't require interface changes
- **The second design** was noticeably better than the first
Remember: The goal is not just working code, but code that minimizes complexity for future developers (including yourself).