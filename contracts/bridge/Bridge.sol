pragma solidity ^0.4.24;

import "../openzeppelin-solidity/contracts/math/SafeMath.sol";
import "../openzeppelin-solidity/contracts/token/ERC20/IERC20.sol";
import "../openzeppelin-solidity/contracts/token/ERC721/IERC721.sol";
import "../openzeppelin-solidity/contracts/ownership/Ownable.sol";
import "../servicechain_nft/INFTReceiver.sol";
import "../servicechain_token/ITokenReceiver.sol";

contract Bridge is ITokenReceiver, INFTReceiver, Ownable {
    uint public constant  version = 1;
    address public counterpartBridge;
    bool public isRunning;

    mapping (address => address) public allowedTokens; // <token, counterpart token>

    using SafeMath for uint256;

    uint64 public requestNonce;
    uint64 public handleNonce;

    uint64 public lastHandledRequestBlockNumber;

    enum TokenKind {
        KLAY,
        TOKEN,
        NFT
    }

    constructor () public payable {
        isRunning = true;
    }

    /**
     * Event to log the withdrawal of a token from the Bridge.
     * @param kind The type of token withdrawn (KLAY/TOKEN/NFT).
     * @param from is the requester of the request value transfer event.
     * @param contractAddress Address of token contract the token belong to.
     * @param amount is the amount for KLAY/TOKEN and the NFT ID for NFT.
     * @param requestNonce is the order number of the request value transfer.
     */
    event RequestValueTransfer(TokenKind kind,
        address from,
        uint256 amount,
        address contractAddress,
        address to,
        uint64 requestNonce);

    /**
     * Event to log the withdrawal of a token from the Bridge.
     * @param owner Address of the entity that made the withdrawal.ga
     * @param kind The type of token withdrawn (KLAY/TOKEN/NFT).
     * @param contractAddress Address of token contract the token belong to.
     * @param value For KLAY/TOKEN this is the amount.
     * @param handleNonce is the order number of the handle value transfer.
     */
    event HandleValueTransfer(address owner,
        TokenKind kind,
        address contractAddress,
        uint256 value,
        uint64 handleNonce);

    // start allows the value transfer request.
    function start()
    onlyOwner
    external
    {
        isRunning = true;
    }

    // stop prevent the value transfer request.
    function stop()
    onlyOwner
    external
    {
        isRunning = false;
    }

    // stop prevent the value transfer request.
    function setCounterPartBridge(address _bridge)
    onlyOwner
    external
    {
        counterpartBridge = _bridge;
    }

    // registerToken can update the allowed token with the counterpart token.
    function registerToken(address _token, address _cToken)
    onlyOwner
    external
    {
        allowedTokens[_token] = _cToken;
    }

    // deregisterToken can remove the token in allowedToken list.
    function deregisterToken(address _token)
    onlyOwner
    external
    {
        delete allowedTokens[_token];
    }

    // handleTokenTransfer sends the token by the request.
    function handleTokenTransfer(uint256 _amount, address _to, address _contractAddress, uint64 _requestNonce, uint64 _requestBlockNumber)
    onlyOwner
    external
    {
        require(handleNonce == _requestNonce, "mismatched handle / request nonce");

        IERC20(_contractAddress).transfer(_to, _amount);
        emit HandleValueTransfer(_to, TokenKind.TOKEN, _contractAddress, _amount, handleNonce);

        lastHandledRequestBlockNumber = _requestBlockNumber;

        handleNonce++;
    }

    // handleKLAYTransfer sends the KLAY by the request.
    function handleKLAYTransfer(uint256 _amount, address _to, uint64 _requestNonce, uint64 _requestBlockNumber)
    onlyOwner
    external
    {
        require(handleNonce == _requestNonce, "mismatched handle / request nonce");

        _to.transfer(_amount); // ensure it's not reentrant
        emit HandleValueTransfer(_to, TokenKind.KLAY, address(0), _amount, handleNonce);

        lastHandledRequestBlockNumber = _requestBlockNumber;

        handleNonce++;
    }

    // handleNFTTransfer sends the NFT by the request.
    function handleNFTTransfer(uint256 _uid, address _to, address _contractAddress, uint64 _requestNonce, uint64 _requestBlockNumber)
    onlyOwner
    external
    {
        require(handleNonce == _requestNonce, "mismatched handle / request nonce");

        IERC721(_contractAddress).safeTransferFrom(address(this), _to, _uid);
        emit HandleValueTransfer(_to, TokenKind.NFT, _contractAddress, _uid, handleNonce);

        lastHandledRequestBlockNumber = _requestBlockNumber;

        handleNonce++;
    }

    //////////////////////////////////////////////////////////////////////////////
    // Receiver functions of Token for 1-step deposits to the Bridge
    bytes4 constant TOKEN_RECEIVED = 0xbc04f0af;

    function onTokenReceived(address _from, uint256 _amount, address _to)
    public
    returns (bytes4)
    {
        require(isRunning, "stopped bridge");
        require(allowedTokens[msg.sender] != address(0), "Not a valid token");
        require(_amount > 0, "zero amount");
        emit RequestValueTransfer(TokenKind.TOKEN, _from, _amount, msg.sender, _to, requestNonce);
        requestNonce++;
        return TOKEN_RECEIVED;
    }

    // Receiver function of NFT for 1-step deposits to the Bridge
    bytes4 private constant ERC721_RECEIVED = 0x150b7a02;

    function onNFTReceived(
        address from,
        uint256 tokenId,
        address to
    )
    public
    returns(bytes4)
    {
        require(isRunning, "stopped bridge");
        require(allowedTokens[msg.sender] != address(0), "Not a valid token");

        emit RequestValueTransfer(TokenKind.NFT, from, tokenId, msg.sender, to, requestNonce);
        requestNonce++;
        return ERC721_RECEIVED;
    }

    // () requests transfer KLAY to msg.sender address on relative chain.
    function () external payable {
        require(isRunning, "stopped bridge");
        require(msg.value > 0, "zero msg.value");

        emit RequestValueTransfer(TokenKind.KLAY, msg.sender, msg.value, address(0), msg.sender, requestNonce);
        requestNonce++;
    }

    // requestKLAYTransfer requests transfer KLAY to _to on relative chain.
    function requestKLAYTransfer(address _to) external payable {
        require(isRunning, "stopped bridge");
        require(msg.value > 0, "zero msg.value");

        emit RequestValueTransfer(TokenKind.KLAY, msg.sender, msg.value, address(0), _to, requestNonce);
        requestNonce++;
    }

    // chargeWithoutEvent sends KLAY to this contract without event for increasing the withdrawal limit.
    function chargeWithoutEvent() external payable {
    }
    //////////////////////////////////////////////////////////////////////////////
}
