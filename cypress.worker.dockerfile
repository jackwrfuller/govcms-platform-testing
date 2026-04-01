FROM cypress/included:13.13.0

WORKDIR /app

# Copy worker script
COPY worker/package.json /app/package.json
COPY worker/index.js /app/index.js

# Copy Cypress config and test specs
COPY cypress/ /app/cypress/

ENV APP_URL=http://app:3000
ENV POLL_INTERVAL=5000
ENV CYPRESS_CONFIG=/app/cypress/cypress.config.js

ENTRYPOINT ["node", "index.js"]
