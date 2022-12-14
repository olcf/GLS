#include<iostream>
#include "attr_check.h"
#include<cstring>
#include<gpfs.h>
#include<vector>
using namespace std;

vector<char> clean(char* buf, int size) {
	vector<char> v;
	for (int i = 0; i < size; i++) {
		char c = buf[i];
		if (isprint(c)) {
			v.push_back(c);
		}
		if ((int)c == 1) {
			v.push_back('|');
		}
	}
	return v;
}

extern "C" {
int print() {
	cout << "Test from C++" << endl;
	return 42;
}
}


/*
 * Returns:
 * 0: File is resident
 * 1: File is premigrated
 * 2: file is migrated
 */
int attr_check(char* path) {
	FILE* f = fopen(path, "rb");

	char* buffer = (char*) calloc(1024, sizeof(char));
	int attrSize = 0;
	int rc = gpfs_fgetattrs(fileno(f), GPFS_ATTRFLAG_INCL_DMAPI, buffer, 1024*sizeof(char), &attrSize);

	vector<char> vectorized_buf = clean(buffer, attrSize);
	string str = "";
	for (auto c: vectorized_buf) {
		str += c;
	}

	if (str.find("IBMTPS") != string::npos) {
		if (str.find("IBMPMig") != string::npos) {
			//cout << "File is premigrated" << endl;
			return 1;
		} else {
		   	//cout << "File has been migrated" << endl;
			return 2;
		}
	}

	fclose(f);
	//cout << "File is resident" << endl;
	return 0;
}
