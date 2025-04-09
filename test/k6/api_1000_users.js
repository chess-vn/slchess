import { check } from "k6";
import http from "k6/http";
import { sleep } from "k6";

// Configuration: Number of virtual users and duration of the test for 100 users
export const options = {
  stages: [
    { duration: "5m", target: 1000 }, // Ramp up to 1000 users over 5 minutes
    { duration: "10m", target: 1000 }, // Hold 1000 users for 10 minutes
    { duration: "5m", target: 0 }, // Gradually scale down to 0 users
  ],
  thresholds: {
    http_req_failed: ["rate<0.02"], // Less than 2% errors
    http_req_duration: ["p(95)<2000"], // 95% of requests < 2s
  },
  discardResponseBodies: true,
};

const endpoints = [
  "/user",
  "/userRatings",
  "/matchResults",
  "/activeMatches",
  "/friends",
];

const baseUrl = __ENV.BASE_URL;
const token = __ENV.TOKEN;

// List of endpoints to hit

export default function main() {
  const randomPath = endpoints[Math.floor(Math.random() * endpoints.length)];
  const url = baseUrl + randomPath;

  const headers = {
    Authorization: `${token}`,
    "Content-Type": "application/json",
  };

  const res = http.get(url, { headers });

  check(res, {
    "status is 200": (r) => r.status === 200,
    "response time < 500ms": (r) => r.timings.duration < 500,
  });

  sleep(10); // Simulate think time
}
