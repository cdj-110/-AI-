cmake_minimum_required(VERSION 3.16)
project(wk_opcda VERSION 0.1.0 LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)
find_package(Threads REQUIRED)

add_library(wk_opcda SHARED
    api/wk_opcda.cpp
    auth/crypto.cpp
    auth/ntlm.cpp
    dcom/activation.cpp
    dcom/object_reference.cpp
    opcda/browse.cpp
    opcda/item_management.cpp
    opcda/connection_point.cpp
    opcda/data_callback.cpp
    opcda/server.cpp
    opcda/sync_io.cpp
    rpc/endpoint_mapper.cpp
    rpc/callback_server.cpp
    rpc/ndr.cpp
    rpc/rpc_pdu.cpp
    rpc/rpc_transport.cpp
)

target_include_directories(wk_opcda
    PUBLIC ${CMAKE_CURRENT_SOURCE_DIR}/api
    PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}
)

target_compile_options(wk_opcda PRIVATE -Wall -Wextra -Wpedantic)
target_link_libraries(wk_opcda PUBLIC Threads::Threads)
set_target_properties(wk_opcda PROPERTIES OUTPUT_NAME wk_opcda VERSION 0.1.0 SOVERSION 0)

add_executable(wk_opcda_connect_test examples/connect_test.c)
target_link_libraries(wk_opcda_connect_test PRIVATE wk_opcda)
target_include_directories(wk_opcda_connect_test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})

add_executable(wk_opcda_callback_auth_test examples/callback_auth_test.cpp)
target_link_libraries(wk_opcda_callback_auth_test PRIVATE wk_opcda)
target_include_directories(wk_opcda_callback_auth_test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})

include(CTest)
if(BUILD_TESTING)
    add_executable(wk_opcda_ping_test tests/ping_test.cpp rpc/ndr.cpp)
    target_include_directories(wk_opcda_ping_test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
    target_compile_options(wk_opcda_ping_test PRIVATE -UNDEBUG)
    add_test(NAME wk_opcda_ping_test COMMAND wk_opcda_ping_test)
    add_executable(wk_opcda_rpc_pdu_test
        tests/rpc_pdu_test.cpp
        auth/crypto.cpp
        auth/ntlm.cpp
        dcom/activation.cpp
        dcom/object_reference.cpp
        opcda/browse.cpp
        opcda/item_management.cpp
        opcda/connection_point.cpp
        opcda/data_callback.cpp
        opcda/server.cpp
        opcda/sync_io.cpp
        rpc/endpoint_mapper.cpp
        rpc/ndr.cpp
        rpc/rpc_pdu.cpp
    )
    target_include_directories(wk_opcda_rpc_pdu_test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
    target_compile_options(wk_opcda_rpc_pdu_test PRIVATE -Wall -Wextra -Wpedantic)
    add_test(NAME wk_opcda_rpc_pdu_test COMMAND wk_opcda_rpc_pdu_test)

    add_executable(wk_opcda_variant_data_integrity_test
        tests/variant_data_integrity_test.cpp
        dcom/object_reference.cpp
        opcda/sync_io.cpp
        rpc/ndr.cpp
        rpc/rpc_pdu.cpp
        auth/crypto.cpp
        auth/ntlm.cpp
    )
    target_include_directories(wk_opcda_variant_data_integrity_test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
    target_compile_options(wk_opcda_variant_data_integrity_test PRIVATE -Wall -Wextra -Wpedantic)
    add_test(NAME wk_opcda_variant_data_integrity_test COMMAND wk_opcda_variant_data_integrity_test)

    if(UNIX)
        add_executable(wk_opcda_native_state_test tests/native_state_test.cpp)
        target_include_directories(wk_opcda_native_state_test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
        target_compile_options(wk_opcda_native_state_test PRIVATE -UNDEBUG)
        target_link_libraries(wk_opcda_native_state_test PRIVATE wk_opcda)
        add_test(NAME wk_opcda_native_state_test COMMAND wk_opcda_native_state_test)
        add_executable(wk_opcda_socket_utils_test tests/socket_utils_test.cpp)
        target_include_directories(wk_opcda_socket_utils_test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
        target_compile_options(wk_opcda_socket_utils_test PRIVATE -Wall -Wextra -Wpedantic)
        add_test(NAME wk_opcda_socket_utils_test COMMAND wk_opcda_socket_utils_test)
    endif()
endif()

install(TARGETS wk_opcda LIBRARY DESTINATION lib)
install(FILES api/wk_opcda.h DESTINATION include)
