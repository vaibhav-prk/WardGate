import http from "k6/http";
import { check, sleep } from "k6";
import { generateHeaders } from "./auth_helper.js";

export const options = {
  stages: [
    { duration: "3s", target: 10 },
    { duration: "5s", target: 50 },
    { duration: "2s", target: 0 },
  ],
  thresholds: {
    checks: ["rate==1.0"],
  },
};

const BASE_URL = __ENV.TARGET_URL || "http://localhost:8080";

export default function () {
  const path = "/";
  const headers = generateHeaders("GET", path, "");

  const res = http.get(`${BASE_URL}${path}`, {
    headers,
    responseCallback: http.expectedStatuses(200, 404, 429),
  });

  check(res, {
    "valid gateway response": (r) =>
      r.status === 200 || r.status === 404 || r.status === 429,
  });

  sleep(0.05);
}
