#include <unistd.h>

int wk_run_variant_data_integrity_test(void);

int main(void) {
    static const char passed[] = "ARM VARIANT data integrity test passed\n";
    const int result = wk_run_variant_data_integrity_test();
    if (result == 0) write(STDOUT_FILENO, passed, sizeof(passed) - 1);
    return result;
}
