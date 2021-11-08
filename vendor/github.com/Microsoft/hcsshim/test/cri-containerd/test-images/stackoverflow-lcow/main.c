#include <stdio.h>

void foo(){
    foo();
}

int main() {
  foo();
  return 0;
}