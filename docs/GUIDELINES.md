To maintain a clean codebase, please follow these guidelines:

1. **Write self-documenting code:** Keep comments to a bare minimum. If the code cannot explain itself, it needs to be refactored.
2. **Test-Driven Development:** Write a comprehensive test for any testable logic *before* writing the implementation to satisfy it.
3. **Keep files small and decoupled:** Aim for self-contained, minimally interdependent modules. This strikes the right balance between strict minimalism and bloated feature sets, ensuring the code remains maintainable.
4. **Balance brevity with performance:** Favor doing more with less code, provided there are no drawbacks. If leaner code compromises performance, weigh the trade-offs carefully. Both simplicity and speed are valuable.
5. **Credit referenced code:** When referencing or adapting code from another project, add an entry to `docs/CREDITS.md` with the project name, author, license, and URL. This does not apply to our own prior projects.
