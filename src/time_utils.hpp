
/**
 * @file:       time_utils.hpp
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


#pragma once
#ifndef ADRM_TIME_UTILS_HPP_
#define ADRM_TIME_UTILS_HPP_

#include <cstdint>
#include <ctime>
#include <string>

namespace adrm {


    struct Duration {
        std::int64_t days = 0;
        std::int64_t hours = 0;
        std::int64_t minutes = 0;
        std::int64_t seconds = 0;

        [[nodiscard]] auto toSeconds() const noexcept -> std::int64_t;
    };

    enum class FilterOp : std::uint8_t
    {
        exact,
        gte,
        lte,
    };

    struct DateFilter {
        FilterOp op = FilterOp::exact;
        bool is_relative = true;
        Duration relative;
        std::tm absolute {};
    };

    struct SizeFilter {
        FilterOp op = FilterOp::exact;
        std::uint64_t bytes = 0;
    };

    [[nodiscard]] auto isDurationSpecifier(const std::string & s) -> bool;

    [[nodiscard]] auto parseDuration(const std::string & s) -> Duration;

    [[nodiscard]] auto durationToDeadline(const Duration & dur) -> std::string;

    [[nodiscard]] auto nowAsString() -> std::string;

    [[nodiscard]] auto timeTToDateString(std::time_t t) -> std::string;

    [[nodiscard]] auto parseAbsoluteDate(const std::string & s) -> std::tm;

    [[nodiscard]] auto tmToString(const std::tm & t) -> std::string;

    [[nodiscard]] auto timeTToTm(std::time_t t) -> std::tm;

    [[nodiscard]] auto tmToTimeT(const std::tm & t) -> std::time_t;

    [[nodiscard]] auto parseSizeSpec(const std::string & s) -> SizeFilter;

    [[nodiscard]] auto parseDateFilter(const std::string & s) -> DateFilter;

    [[nodiscard]] auto formatSizeHuman(std::uint64_t bytes) -> std::string;

    [[nodiscard]] auto deadlineFromNow(std::int64_t default_days) -> std::string;


} // namespace adrm


#endif
