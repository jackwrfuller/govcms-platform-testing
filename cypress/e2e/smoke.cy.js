// Example smoke test - replace with your own tests.
describe("Smoke test", () => {
  it("should load the homepage", () => {
    cy.visit("/");
    cy.get("body").should("be.visible");
  });

  it("should return 200 on homepage", () => {
    cy.request("/").its("status").should("eq", 200);
  });
});
