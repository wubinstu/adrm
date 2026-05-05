
# @file:       CMakeLists.txt
# @author:     GLM-5.1-OpenCode
# @date:       2026-05-05
# @license:    MIT License
#
# Copyright (c) 2026 GLM-5.1-OpenCode
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.
#
# DISCLAIMER:
# 1. User acknowledges all application risks and consequences arising from usage
#    of this software are exclusively borne by the user.
# 2. The original author shall not be held liable for any direct, indirect,
#    incidental, special, exemplary or consequential damages.
# 3. No warranty of fitness for particular purpose or non-infringement is provided.


cmake_minimum_required(VERSION 3.7...3.27)

project(adrm
        DESCRIPTION "Advance Remover"
        VERSION 1.1.0
        LANGUAGES CXX)

set(CMAKE_EXPORT_COMPILE_COMMANDS ON)
if (DEFINED CMAKE_BUILD_TYPE AND CMAKE_BUILD_TYPE STREQUAL "Debug")
    set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -Wall -Wextra -g -O0")
elseif(DEFINED CMAKE_BUILD_TYPE AND CMAKE_BUILD_TYPE STREQUAL "Release")
    set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -O2")
endif()

set(CMAKE_C_STANDARD 23)
set(CMAKE_CXX_STANDARD 23)


string(TIMESTAMP BUILD_TIMESTAMP "%Y-%m-%d %H:%M:%S")
message(STATUS "Build TimeStamp = ${BUILD_TIMESTAMP}")
message(STATUS "System Name = ${CMAKE_SYSTEM_NAME}")
message(STATUS "System Processor = ${CMAKE_SYSTEM_PROCESSOR}")
message(STATUS "C Compiler(F) = ${CMAKE_C_COMPILER}${CMAKE_C_FLAGS}")
message(STATUS "C++ Compiler(F) = ${CMAKE_CXX_COMPILER}${CMAKE_CXX_FLAGS}")
message(STATUS "Assembler = ${CMAKE_ASM_COMPILER}")
message(STATUS "Linker = ${CMAKE_LINKER}")
message(STATUS "Archiver = ${CMAKE_AR}")
message(STATUS "Make Program = ${CMAKE_MAKE_PROGRAM}")
message(STATUS "Project Name = ${CMAKE_PROJECT_NAME}")
message(STATUS "Project Version = ${CMAKE_PROJECT_VERSION}")
message(STATUS "Project Build Type = ${CMAKE_BUILD_TYPE}")
message(STATUS "Project Source Path = ${CMAKE_SOURCE_DIR}")
message(STATUS "C Language Standard = ${CMAKE_C_STANDARD}")
message(STATUS "CXX Language Standard = ${CMAKE_CXX_STANDARD}")
message(STATUS "Binary OutPut Path = ${PROJECT_BINARY_DIR}/${EXECUTABLE_OUTPUT_PATH}")
message(STATUS "Library Output Path = ${PROJECT_BINARY_DIR}/${LIBRARY_OUTPUT_PATH}")
message(STATUS "Install Path = ${CMAKE_INSTALL_PREFIX}")


set(ADRM_SOURCES
    src/main.cpp
    src/cli.cpp
    src/config.cpp
    src/database.cpp
    src/file_ops.cpp
    src/filter.cpp
    src/ignore.cpp
    src/time_utils.cpp
    src/trash_ops.cpp
)

add_executable(adrm ${ADRM_SOURCES})

target_include_directories(adrm PRIVATE ${CMAKE_SOURCE_DIR}/src)

option(ADRM_STATIC_LINK "Statically link SQLite3 and C++ runtime" OFF)

if(ADRM_STATIC_LINK)
    set(CMAKE_FIND_LIBRARY_SUFFIXES .a)
    set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -static-libstdc++ -static-libgcc")
endif()

find_package(SQLite3 REQUIRED)
target_link_libraries(adrm PRIVATE SQLite::SQLite3)
target_link_libraries(adrm PRIVATE dl pthread)

target_compile_definitions(adrm PRIVATE
    ADRM_VERSION_MAJOR=${PROJECT_VERSION_MAJOR}
    ADRM_VERSION_MINOR=${PROJECT_VERSION_MINOR}
    ADRM_VERSION_PATCH=${PROJECT_VERSION_PATCH}
)
