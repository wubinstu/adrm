/**
 * @file:       time_utils.cpp
 * @author:     GLM-5.1-OpenCode
 *
 * @date:       2026-04-30
 * @license:    MIT License
 *
 * Copyright (c) 2026 GLM-5.1-OpenCode
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 *
 * DISCLAIMER:
 * 1. User acknowledges all application risks and consequences arising from usage
 *    of this software are exclusively borne by the user.
 * 2. The original author shall not be held liable for any direct, indirect,
 *    incidental, special, exemplary or consequential damages.
 * 3. No warranty of fitness for particular purpose or non-infringement is provided. */


#include "time_utils.hpp"

#include <chrono>
#include <cstring>

namespace adrm {


    auto Duration::toSeconds() const noexcept -> std::int64_t {
        return this->days * 86400 + this->hours * 3600 + this->minutes * 60 + this->seconds;
    }

    auto isDurationSpecifier(const std::string & s) -> bool {
        if (s.empty())
            return false;

        std::size_t pos = 0;
        if (s[pos] == '+')
            pos = 1;
        if (pos >= s.size())
            return false;

        bool has_unit = false;
        while (pos < s.size()) {
            if (s[pos] < '0' || s[pos] > '9')
                return false;

            while (pos < s.size() && s[pos] >= '0' && s[pos] <= '9')
                ++pos;

            if (pos >= s.size())
                break;

            auto c = s[pos];
            if (c == 'd' || c == 'h' || c == 'm' || c == 's') {
                has_unit = true;
                ++pos;
            }
            else {
                return false;
            }
        }

        return has_unit;
    }

    auto parseDuration(const std::string & s) -> Duration {
        Duration dur;
        std::size_t pos = 0;

        if (pos < s.size() && s[pos] == '+')
            ++pos;

        while (pos < s.size()) {
            std::int64_t num = 0;
            while (pos < s.size() && s[pos] >= '0' && s[pos] <= '9') {
                num = num * 10 + (s[pos] - '0');
                ++pos;
            }
            if (pos >= s.size())
                break;

            switch (s[pos]) {
            case 'd':
                dur.days = num;
                break;
            case 'h':
                dur.hours = num;
                break;
            case 'm':
                dur.minutes = num;
                break;
            case 's':
                dur.seconds = num;
                break;
            default:
                break;
            }
            ++pos;
        }

        return dur;
    }

    auto durationToDeadline(const Duration & dur) -> std::string {
        auto now = std::chrono::system_clock::now();
        auto deadline = now + std::chrono::seconds(dur.toSeconds());
        auto t = std::chrono::system_clock::to_time_t(deadline);
        return timeTToDateString(t);
    }

    auto nowAsString() -> std::string {
        auto now = std::chrono::system_clock::now();
        auto t = std::chrono::system_clock::to_time_t(now);
        return timeTToDateString(t);
    }

    auto timeTToDateString(std::time_t t) -> std::string {
        auto tm = std::localtime(&t);
        if (!tm)
            return "unknown";
        char buf[32];
        std::strftime(buf, sizeof(buf), "%Y-%m-%d %H:%M:%S", tm);
        return std::string(buf);
    }

    auto parseAbsoluteDate(const std::string & s) -> std::tm {
        std::tm t {};
        std::memset(&t, 0, sizeof(t));
        t.tm_year = 0;
        t.tm_mon = 0;
        t.tm_mday = 1;

        std::string cleaned;
        for (auto c : s) {
            if (c == '/' || c == '_' || c == '-' || c == ':')
                cleaned += ' ';
            else
                cleaned += c;
        }

        int year = 0, month = 0, day = 1;
        int hour = 0, min = 0, sec = 0;
        auto n = std::sscanf(cleaned.c_str(), "%d %d %d %d %d %d",
                             &year, &month, &day, &hour, &min, &sec);

        t.tm_year = year - 1900;
        t.tm_mon = (n >= 2) ? month - 1 : 0;
        t.tm_mday = (n >= 3) ? day : 1;
        t.tm_hour = (n >= 4) ? hour : 0;
        t.tm_min = (n >= 5) ? min : 0;
        t.tm_sec = (n >= 6) ? sec : 0;
        t.tm_isdst = -1;

        return t;
    }

    auto tmToString(const std::tm & t) -> std::string {
        char buf[32];
        std::strftime(buf, sizeof(buf), "%Y-%m-%d %H:%M:%S", &t);
        return std::string(buf);
    }

    auto timeTToTm(std::time_t t) -> std::tm {
        auto ptr = std::localtime(&t);
        if (ptr)
            return *ptr;
        return {};
    }

    auto tmToTimeT(const std::tm & t) -> std::time_t {
        auto copy = t;
        return std::mktime(&copy);
    }

    auto parseSizeSpec(const std::string & s) -> SizeFilter {
        SizeFilter sf;
        sf.bytes = 0;
        sf.op = FilterOp::exact;

        std::size_t pos = 0;
        if (pos < s.size() && s[pos] == '+') {
            sf.op = FilterOp::gte;
            ++pos;
        }
        else if (pos < s.size() && s[pos] == '-') {
            sf.op = FilterOp::lte;
            ++pos;
        }

        while (pos < s.size()) {
            std::uint64_t num = 0;
            while (pos < s.size() && s[pos] >= '0' && s[pos] <= '9') {
                num = num * 10 + static_cast<std::uint64_t>(s[pos] - '0');
                ++pos;
            }
            if (pos >= s.size())
                break;

            auto c = s[pos];
            ++pos;
            switch (c) {
            case 't':
                sf.bytes += num * 1099511627776ULL;
                break;
            case 'g':
                sf.bytes += num * 1073741824ULL;
                break;
            case 'm':
                sf.bytes += num * 1048576ULL;
                break;
            case 'k':
                sf.bytes += num * 1024ULL;
                break;
            case 'b':
                sf.bytes += num;
                break;
            default:
                break;
            }
        }

        return sf;
    }

    auto parseDateFilter(const std::string & s) -> DateFilter {
        DateFilter df;
        df.op = FilterOp::exact;
        df.is_relative = true;

        std::size_t pos = 0;
        if (pos < s.size() && s[pos] == '+') {
            df.op = FilterOp::gte;
            ++pos;
        }
        else if (pos < s.size() && s[pos] == '-') {
            bool might_be_date = false;
            for (std::size_t i = pos + 1; i < s.size(); ++i) {
                auto c = s[i];
                if (c == '/' || c == '_' || c == '-' || c == ':' || c == ' ') {
                    might_be_date = true;
                    break;
                }
            }
            if (might_be_date) {
                df.op = FilterOp::lte;
                ++pos;
            }
            else {
                df.op = FilterOp::lte;
                ++pos;
            }
        }

        auto rest = s.substr(pos);

        bool looks_like_date = false;
        for (auto c : rest) {
            if (c == '/' || c == '_' || c == ':' || c == ' ') {
                looks_like_date = true;
                break;
            }
            if (c == '-' && !rest.empty()) {
                auto dash_count = 0;
                for (auto ch : rest) {
                    if (ch == '-')
                        ++dash_count;
                }
                if (dash_count >= 2) {
                    looks_like_date = true;
                    break;
                }
            }
        }

        if (looks_like_date) {
            df.is_relative = false;
            df.absolute = parseAbsoluteDate(rest);
        }
        else {
            df.is_relative = true;
            df.relative = parseDuration(rest);
        }

        return df;
    }

    auto formatSizeHuman(std::uint64_t bytes) -> std::string {
        if (bytes >= 1099511627776ULL) {
            auto v = static_cast<double>(bytes) / 1099511627776.0;
            char buf[64];
            std::snprintf(buf, sizeof(buf), "%.2f TB", v);
            return std::string(buf);
        }
        if (bytes >= 1073741824ULL) {
            auto v = static_cast<double>(bytes) / 1073741824.0;
            char buf[64];
            std::snprintf(buf, sizeof(buf), "%.2f GB", v);
            return std::string(buf);
        }
        if (bytes >= 1048576ULL) {
            auto v = static_cast<double>(bytes) / 1048576.0;
            char buf[64];
            std::snprintf(buf, sizeof(buf), "%.2f MB", v);
            return std::string(buf);
        }
        if (bytes >= 1024ULL) {
            auto v = static_cast<double>(bytes) / 1024.0;
            char buf[64];
            std::snprintf(buf, sizeof(buf), "%.2f KB", v);
            return std::string(buf);
        }

        return std::to_string(bytes) + " B";
    }

    auto deadlineFromNow(std::int64_t default_days) -> std::string {
        Duration dur;
        dur.days = default_days;
        return durationToDeadline(dur);
    }


} // namespace adrm
