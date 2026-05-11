FROM node:20-alpine AS frontend-build

WORKDIR /app

COPY apps/frontend/package*.json ./
RUN npm install

COPY apps/frontend/ ./

ARG VITE_API_URL=/api
ENV VITE_API_URL=${VITE_API_URL}

RUN npm run build

FROM nginx:1.27-alpine

COPY docker/nginx/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=frontend-build /app/dist /usr/share/nginx/html
