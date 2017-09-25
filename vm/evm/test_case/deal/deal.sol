pragma solidity ^0.4.0;
contract Deal {
    address private signer;
    address public receiver;

    function Deal() {
        signer = msg.sender;
        receiver = 0x4e2d29c060eefc41a4f5330ecb67da94e0eed266;
    }


    function releasePayment() {
        receiver.transfer(this.balance);
    }

    function getReceiver() returns (address retVal)  {
        return receiver;
    }

    function getSigner() returns (address retVal)  {
        return signer;
    }

    function getBalance() returns (uint balance) {
        return receiver.balance;
    }
}