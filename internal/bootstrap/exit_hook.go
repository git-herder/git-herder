// Пакет bootstrap: подмена os.Exit в тестах.
package bootstrap

import "os"

var exitFn = os.Exit
