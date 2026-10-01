import http from "k6/http";
import { check, sleep } from "k6";
import { generateHeaders } from "./auth_helper.js";

export const options = {
  scenarios: {
    steady_traffic: {
      executor: "constant-arrival-rate",
      rate: 50,
      timeUnit: "1s",
      duration: "5s",
      preAllocatedVUs: 5,
      maxVUs: 20,
    },
  },
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
    responseCallback: http.expectedStatuses(200, 404),
  });

  check(res, {
    "passed gateway security": (r) => r.status === 200 || r.status === 404,
  });

  sleep(0.02);
}
