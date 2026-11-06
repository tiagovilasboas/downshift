# Backend

- Validate inputs at the boundary; return explicit errors and never ignore one.
- Keep business rules out of handlers and transport code.
- Make writes that can run twice safe to repeat (idempotent), or guard them.
- Add or update a test for each rule you change, next to the code it covers.
- Do not log request bodies, credentials or personal data.
